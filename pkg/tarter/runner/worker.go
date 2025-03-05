/*
 * Copyright 2025 TomTom N.V.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package runner

import (
	"bytes"
	"fmt"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	t "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type workerAction string

const (
	Start     workerAction = "start"
	Stop      workerAction = "stop"
	Terminate workerAction = "terminate"
	Exit      workerAction = "exit"
)

type worker struct {
	startStopChan chan workerAction
	sigChan       chan os.Signal
	runnerManager *RunnerManager
	runner        *t.Runner
	tartVMName    string
	stopOnce      sync.Once

	cmd     *exec.Cmd
	cmdLock sync.Mutex
	stdout  bytes.Buffer
	stderr  bytes.Buffer
}

func newWorker(runnerManager *RunnerManager, runner *t.Runner) *worker {
	return &worker{
		startStopChan: make(chan workerAction, 1),
		sigChan:       make(chan os.Signal, 1),
		runnerManager: runnerManager,
		runner:        runner,
		tartVMName:    fmt.Sprintf("%s-%s", runner.Config.Name, string(runner.ID)),
	}
}

func (w *worker) run() {
	defer func() {
		logger.Debugf("Removing worker %s", w.runner.ID)
		w.runnerManager.removeWorker(w.runner.ID)
		w.runnerManager.tartClient.CleanupRunnerConfiguration(w.tartVMName)
	}()

runnerLoop:
	for {
		select {
		case action := <-w.startStopChan:
			if action == Start {
				err := w.startRunner()
				if err != nil {
					break runnerLoop
				}
			} else if action == Stop {
				w.stopRunner()
			} else if action == Terminate {
				w.terminateRunner()
			} else if action == Exit {
				break runnerLoop
			} else {
				logger.Errorf("Unknown action: %v", action)
				break runnerLoop
			}
		}
	}
}

func (w *worker) start() {
	select {
	case w.startStopChan <- Start:
	default: // Non-blocking.
	}
}

func (w *worker) stop() {
	w.stopOnce.Do(func() {
		select {
		case w.startStopChan <- Stop:
		default: // Non-blocking.
		}
	})
}

func (w *worker) terminate() {
	select {
	case w.startStopChan <- Terminate:
	default: // Non-blocking.
	}
}

func (w *worker) startRunner() error {
	w.cmdLock.Lock()
	defer w.cmdLock.Unlock()

	if w.runner.Config.JitConfig != "" {
		// check if runner is already registered on github. There is an issue in ACR that
		// it does remove the runner from github when the runner is idle.
		// Runners based on that config will not connect to github and will fail to start.
		runners, err := w.runnerManager.githubClient.GetRunnerByName(w.runner.GhaRunnerName)
		if err != nil {
			logger.Errorf("Failed to get runner <%v> from github", w.runner.GhaRunnerName)
			w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Failed, err.Error())
			return err
		}
		if len(runners.Runners) == 0 {
			logger.Warnf("Runner <%v> from JIT config not found in github", w.runner.GhaRunnerName)
			w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Failed,
				fmt.Sprintf("runner <%v> from JIT config not found in github", w.runner.GhaRunnerName))
			return fmt.Errorf("failed to configure runner from JIT config")
		}
	}

	registrationToken, err := w.runnerManager.githubClient.GetRunnerRegistrationToken()
	if err != nil {
		logger.Errorf("Failed to get registration token: %v", err)
		w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Failed, err.Error())
		return err
	}
	// Setup runner configuration creates config files that will be mounted to the Tart VM
	// at runner startup.
	// It uses short Runner ID due to the limitation of the GHA runner name length.
	ghaRunnerName, err := w.runnerManager.tartClient.SetupRunnerConfiguration(
		w.runnerManager.nodeName,
		string(w.runner.ID.Short()),
		w.tartVMName,
		w.runner.Config,
		registrationToken.GetToken())
	if err != nil {
		logger.Errorf("Failed to setup runner configuration: %v", err)
		w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Failed, err.Error())
		return err
	}

	if w.runner.GhaRunnerName == "" {
		w.runner.GhaRunnerName = ghaRunnerName
	}

	logger.Infof("Starting runner %s with name %s", w.runner.ID, w.runner.GhaRunnerName)
	args := w.runnerManager.tartClient.BuildCMDArguments(w.tartVMName, w.runner.Config)
	w.cmd = w.runnerManager.tartClient.BuildCommand(&w.stdout, &w.stderr, args...)

	logger.Debugf("Exec command: %v", w.cmd.String())

	err = w.cmd.Start()
	if err != nil {
		logger.Errorf("Failed to start runnner %v: %v, output: %v", w.runner.ID, err.Error(), w.stderr.String())
		w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Failed,
			fmt.Sprintf("failed to start runner: %v", w.stderr.String()))
		return err
	} else {
		logger.Infof("Runner %s started", w.runner.ID)
		w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Running)
	}

	go w.watch()

	return nil
}

func (w *worker) watch() {
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- w.cmd.Wait()
	}()

	select {
	case err := <-doneChan:
		if runnerState, exist := w.runnerManager.stateManager.GetRunnerState(w.runner.ID); exist {
			if runnerState.Status == t.Stopping {
				logger.Infof("Runner %s is stopped", w.runner.ID)
				w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Stopped)
			} else if err != nil {
				logger.Errorf("Runner finished with error: %v, output: %v", err, w.stderr.String())
				w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Failed,
					fmt.Sprintf("runner finished with error: %v, output: %v", err, w.stderr.String()))
			} else {
				logger.Infof("Runner %s finished", w.runner.ID)
				w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Finished)
			}
		}
		w.startStopChan <- Exit
	case sig := <-w.sigChan:
		if sig == syscall.SIGINT {
			logger.Debugf("Runner %s interrupted with SIGINT signal", w.runner.ID)
			if err := w.cmd.Process.Signal(syscall.SIGINT); err != nil {
				logger.Errorf("Error sending signal to process: %v", err)
			}
			w.startStopChan <- Exit
		}
		if sig == syscall.SIGTERM {
			logger.Debugf("Runner %s interrupted with SIGTERM signal", w.runner.ID)
			if err := w.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				logger.Errorf("Error sending signal to process: %v", err)
			}
			w.startStopChan <- Exit
		}
	}
}

func (w *worker) stopRunner() {
	w.cmdLock.Lock()
	defer w.cmdLock.Unlock()

	logger.Infof("Stopping runner %s", w.runner.ID)
	w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Stopping)
	isRemoved := removeGhaRunner(w.runnerManager.githubClient, w.runner.GhaRunnerName)
	if isRemoved {
		w.sigChan <- syscall.SIGINT
		w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Stopped)
		return
	}
	logger.Infof("Runner %s is busy. Waiting for runner to finish his job", w.runner.ID)
}

func (w *worker) terminateRunner() {
	w.cmdLock.Lock()
	defer w.cmdLock.Unlock()

	logger.Infof("Terminating runner %s", w.runner.ID)
	w.sigChan <- syscall.SIGTERM
	w.runnerManager.stateManager.UpdateRunnerStatus(w.runner.ID, t.Failed,
		fmt.Sprintf("runner terminaded after faled health, output: %v", w.stderr.String()))
}

func removeGhaRunner(ghClient *ghclient.Client, runnerGhaName string) bool {
	logger.Infof("Checking if runner <%v> is registered on github", runnerGhaName)
	runners, err := ghClient.GetRunnerByName(runnerGhaName)

	if err != nil {
		logger.Errorf("Failed to get runner <%v> from github", runnerGhaName)
		return false
	}

	if len(runners.Runners) == 0 {
		logger.Infof("Runner <%v> not registered on github", runnerGhaName)
		return true
	}

	if *runners.Runners[0].Busy {
		logger.Infof("Runner <%v> is busy", runnerGhaName)
		return false
	} else {
		logger.Infof("Runner <%v> is idle", runnerGhaName)
		err = ghClient.RemoveRunner(*runners.Runners[0].ID)
		if err != nil {
			logger.Errorf("Failed to remove runner <%v> from github. Error: %v", runnerGhaName, err)
			return false
		}
		return true
	}
}
