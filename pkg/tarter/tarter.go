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

package tarter

import (
	"fmt"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/tart"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	pt "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/config"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/events"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/node"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/prober/results"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/runner"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/state"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"time"
)

const (
	heartbeatInterval = 15 * time.Second
	// nodeStatusUpdateRetry specifies how many times Tarter retries when sending heartbeat with node status failed.
	nodeStatusUpdateRetry = 3
)

type Tarter struct {
	// TODO: Consider using link to Config instead of Config itself
	// standalone mode configuration
	config config.Config
	// maintain all probes
	proberManager *prober.ProberManager
	githubClient  *ghclient.Client
	// maintain all requests to Tart cli tool
	tartClient *tart.Client
	// maintain running processes
	runnerManager *runner.RunnerManager
	// TODO: Allow to communicate with StateManager only for Tarter
	// Runners State Machine which store runners state
	StateManager *state.StateManager
	// event bus
	eventBus *events.EventBus
	// controller client for managed mode
	controllerClient *controller.Client
	// manage node status updates
	nodeManager      *node.Manager
	versionInfo      coreVersion.VersionInfo
	tarterServerPort string
	nodeName         string
	nodeIP           string
	configPath       string
}

func NewTarter(tarterConfig config.TarterConfig,
	appConfig config.Config,
	configPath string,
	nodeName string,
	nodeIP string,
	versionInfo coreVersion.VersionInfo,
) (*Tarter, error) {

	ghConfig := config.GetGithubClientConfig(&tarterConfig)
	githubClient, err := ghclient.New(ghConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate GitHub client, %v", err.Error())
	}

	tartClient := tart.NewClient(
		tarterConfig.TartPath,
		tarterConfig.ConfigFolder,
	)

	maxActiveRunners := int(appConfig.GetNodeCapacity().MaxActiveRunners.IntVal)
	eventBus := events.NewEventBus()
	stateMachine := state.NewStateManager(eventBus, maxActiveRunners)

	livenessManager := results.NewManager(pt.Liveness, eventBus)
	startupManager := results.NewManager(pt.Startup, eventBus)

	t := &Tarter{
		nodeName:         nodeName,
		nodeIP:           nodeIP,
		tarterServerPort: tarterConfig.Port,
		versionInfo:      versionInfo,
		githubClient:     githubClient,
		tartClient:       tartClient,
		proberManager:    prober.NewManager(githubClient, stateMachine, livenessManager, startupManager),
		StateManager:     stateMachine,
		runnerManager:    runner.NewRunnerManager(tartClient, githubClient, stateMachine, maxActiveRunners, nodeName),
		eventBus:         eventBus,
		configPath:       configPath,
		config:           appConfig,
	}
	return t, nil
}

func (t *Tarter) startEventHandling(eventHandlers map[string]func(event events.Event)) {
	for eventType, handler := range eventHandlers {
		ch := make(chan events.Event, 10)
		t.eventBus.Subscribe(eventType, ch)
		go func(ch chan events.Event, handler func(event events.Event)) {
			for event := range ch {
				handler(event)
			}
		}(ch, handler)
	}
}

func (t *Tarter) handleRunnerCreated(event events.Event) {
	r := event.Payload.(*tt.Runner)
	logger.Debugf("Tarter handling runner <create> event for runner id: %s", r.ID)
	t.runnerManager.StartRunnerWorker(r)
}

func (t *Tarter) handleRunnerStopping(event events.Event) {
	r := event.Payload.(*tt.Runner)
	logger.Debugf("Tarter handling runner <stopping> event for runner id: %s", r.ID)
	t.runnerManager.StopRunnerWorker(r.ID)
}

func (t *Tarter) handleRunnerLivenessHealthCheck(event events.Event) {
	p := event.Payload.(tt.HealthCheckEventPayload)
	t.StateManager.UpdateRunnersHealth(p.RunnerID, p.HealthResult, pt.Liveness)
	if p.HealthResult == probe.Failure {
		logger.Infof("Runner %v failed liveness probe.", p.RunnerID)
		t.runnerManager.TerminateRunnerWorker(p.RunnerID)
	}
}

func (t *Tarter) handleRunnerStartupHealthCheck(event events.Event) {
	p := event.Payload.(tt.HealthCheckEventPayload)

	t.StateManager.UpdateRunnersHealth(p.RunnerID, p.HealthResult, pt.Startup)
	if p.HealthResult == probe.Failure {
		logger.Infof("Runner %v failed startup probe.", p.RunnerID)
		t.runnerManager.TerminateRunnerWorker(p.RunnerID)
	} else {
		logger.Infof("Runner %v passed startup probe.", p.RunnerID)
	}
}

// restartRunner will send AddRunner request to State Manager with Latest Runner Config stored in a StandaloneConfig
func (t *Tarter) restartRunner(runnerName string) {
	for _, runnerConfig := range t.config.GetRunnersConfig() {
		if runnerConfig.Name == runnerName && runnerConfig.RestartOnFailure {
			logger.Infof("Restarting %v ...", runnerName)
			// Sleep for 15 seconds before restarting the runner to avoid tart cli failures
			time.Sleep(15 * time.Second)
			t.StateManager.AddRunner(tt.Runner{Config: runnerConfig})
		}
	}
}

func (t *Tarter) addProber(runner *tt.Runner) {
	logger.Infof("Adding probe for runner %v ...", runner.ID)
	probeTarget := &pt.ProbeTarget{
		ID:            runner.ID,
		Name:          runner.GhaRunnerName, // GhaRunnerName used by GitHub prober
		Type:          pt.ProbeTargetTypeTartRunner,
		StartupProbe:  runner.Config.StartupProbe,
		LivenessProbe: runner.Config.LivenessProbe,
	}
	t.proberManager.AddProber(probeTarget)
}

func (t *Tarter) ListRunnerWorkers() []string {
	return t.runnerManager.ListWorkers()
}
