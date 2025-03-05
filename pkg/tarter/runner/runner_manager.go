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
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/tart"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/state"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"sync"
)

type RunnerManager struct {
	tartClient *tart.Client

	githubClient *ghclient.Client

	//Worker is the worker
	workers map[runnerKey]*worker
	// Lock for accessing & mutating workers
	workerLock sync.RWMutex

	stateManager *state.StateManager

	maxWorkers int

	nodeName string
}

type runnerKey struct {
	runnerID utils.UID
}

func NewRunnerManager(
	tartClient *tart.Client,
	githubClient *ghclient.Client,
	stateManager *state.StateManager,
	maxWorkers int,
	nodeName string) *RunnerManager {
	return &RunnerManager{
		workers:      make(map[runnerKey]*worker),
		tartClient:   tartClient,
		githubClient: githubClient,
		stateManager: stateManager,
		maxWorkers:   maxWorkers,
		nodeName:     nodeName,
	}
}

func (m *RunnerManager) StartRunnerWorker(runner *tt.Runner) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	key := runnerKey{
		runnerID: runner.ID,
	}
	if _, ok := m.workers[key]; ok {
		logger.Errorf("Runner Worker already exists for runner %s", runner.ID)
		return
	}
	if len(m.workers) >= m.maxWorkers {
		logger.Errorf("Max workers limit reached. Cannot start new worker for runner %s", runner.ID)
		return
	}
	w := newWorker(m, runner)
	m.workers[key] = w
	go w.run()
	w.start()
}

func (m *RunnerManager) StopRunnerWorker(runnerID utils.UID) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	key := runnerKey{
		runnerID: runnerID,
	}
	if w, ok := m.workers[key]; ok {
		w.stop()
	} else {
		logger.Errorf("Runner Worker does not exist for runner %s. Stop runner worker failed", runnerID)
	}
}

func (m *RunnerManager) TerminateRunnerWorker(runnerID utils.UID) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	key := runnerKey{
		runnerID: runnerID,
	}
	if w, ok := m.workers[key]; ok {
		w.terminate()
	} else {
		logger.Errorf("Runner Worker does not exist for runner %s. Terminate runner worker failed", runnerID)
	}
}

func (m *RunnerManager) getWorker(runnerID utils.UID) (*worker, bool) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	worker, ok := m.workers[runnerKey{runnerID}]
	return worker, ok
}

// Called by the worker after exiting.
func (m *RunnerManager) removeWorker(runnerID utils.UID) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, runnerKey{runnerID})
}

// workerCount returns the total number of probe workers. For testing.
func (m *RunnerManager) workerCount() int {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	return len(m.workers)
}

// ListWorkers returns a list of all active workers. For testing.
func (m *RunnerManager) ListWorkers() []string {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	workers := make([]string, 0, len(m.workers))
	for k, _ := range m.workers {
		workers = append(workers, string(k.runnerID))
	}
	return workers
}
