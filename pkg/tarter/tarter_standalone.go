package tarter

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/config"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/events"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
)

func (t *Tarter) readStandaloneConfig(configPath string) {
	standaloneConfig := config.ReadStandaloneConfiguration(configPath)
	t.config = &standaloneConfig
}

func (t *Tarter) StartStandaloneTarter() {
	logger.Infof("Initializing Standalone Tarter")

	if t.config == nil {
		logger.Fatalf("Standalone configuration is not loaded")
	}

	eventHandlers := map[string]func(event events.Event){
		tt.EventRunnerCreated:             t.handleRunnerCreated,
		tt.EventRunnerStopRequest:         t.handleRunnerStopping,
		tt.EventRunnerStatusUpdated:       t.handleStandaloneRunnerStatusUpdate,
		tt.EventRunnerLivenessHealthCheck: t.handleRunnerLivenessHealthCheck,
		tt.EventRunnerStartupHealthCheck:  t.handleRunnerStartupHealthCheck,
	}

	t.startEventHandling(eventHandlers)

	for _, runnerConfig := range t.config.GetRunnersConfig() {
		t.StateManager.AddRunner(tt.Runner{Config: runnerConfig})
	}
}

func (t *Tarter) handleStandaloneRunnerStatusUpdate(event events.Event) {
	r := event.Payload.(*tt.Runner)
	if runnerState, exist := t.StateManager.GetRunnerState(r.ID); exist {
		logger.Debugf("Tarter handling runner <%s> status update event for runner id: %s",
			runnerState.Status, r.ID)

		if runnerState.Status == tt.Finished || runnerState.Status == tt.Failed {
			go t.restartRunner(r.Config.Name)
		}

		if runnerState.Status == tt.Running {
			if r.Config.LivenessProbe != nil || r.Config.StartupProbe != nil {
				t.addProber(r)
			}
		}
	}
}

func (t *Tarter) GetStandaloneConfig() config.Config {
	return t.config
}

func (t *Tarter) RestartStandaloneRunners() {
	logger.Debugf("Restarting standalone mode form configuration")
	t.readStandaloneConfig(t.configPath)
	activeRunners := t.StateManager.ListActiveRunners()
	if len(activeRunners) >= int(t.config.GetNodeCapacity().MaxActiveRunners.IntVal) {
		logger.Debugf("Maximum number of runners already active")
		return
	}
	activeRunnerNames := make(map[string]bool)
	for _, activeRunner := range activeRunners {
		activeRunnerNames[activeRunner.Name] = true
	}

	for _, runnerConfig := range t.config.GetRunnersConfig() {
		if _, exists := activeRunnerNames[runnerConfig.Name]; !exists {
			t.StateManager.AddRunner(tt.Runner{Config: runnerConfig})
		}
	}
}
