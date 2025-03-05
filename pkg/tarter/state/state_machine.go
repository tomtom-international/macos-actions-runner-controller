package state

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	pt "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/events"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"strings"
	"sync"
	"time"
)

const (
	// Maximum number of runners that can be created and stored in State
	// at the same time.
	maxRunnersInState = 25
)

// StateManager is a State Machine to manage runner states.
type StateManager struct {
	// runners contains all runners that are exist in the State Manager.
	runners map[utils.UID]*tt.RunnerState

	// activeRunners contains all runners that are currently in running state.
	activeRunners map[utils.UID]*tt.RunnerState

	eventBus         *events.EventBus
	maxActiveRunners int
	mu               sync.RWMutex
}

func NewStateManager(eventBus *events.EventBus, maxActiveRunners int) *StateManager {
	return &StateManager{
		runners:          make(map[utils.UID]*tt.RunnerState),
		activeRunners:    make(map[utils.UID]*tt.RunnerState),
		maxActiveRunners: maxActiveRunners,
		eventBus:         eventBus,
	}
}

// AddRunner adds new runner to the state.
func (sm *StateManager) AddRunner(runner tt.Runner) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if maximum number of runners reached and remove oldest exited runner
	if len(sm.runners) >= maxRunnersInState {
		var oldest *tt.RunnerState
		for _, runner := range sm.runners {
			if (oldest == nil || runner.CreatedAt.Before(oldest.CreatedAt)) &&
				(runner.Status != tt.Running && runner.Status != tt.Stopping) {
				oldest = runner
			}
		}
		if oldest != nil {
			delete(sm.runners, oldest.Runner.ID)
		}
	}

	// TODO: Implement host node resource check before place new runner to avoid resource starvation

	if len(sm.activeRunners) >= sm.maxActiveRunners {
		logger.Warnf("Maximum number of active runners reached, cannot add new runner")
		return
	}

	if runner.ID == "" {
		runner.ID = utils.NewUUID()
	}

	runnerState := &tt.RunnerState{
		Runner:    &runner,
		Status:    tt.Created,
		CreatedAt: time.Now(),
	}
	logger.Infof("Adding runner %s", runner.ID)
	if r, ok := sm.runners[runner.ID]; ok {
		if r.Status != tt.Failed {
			logger.Warnf("Runner with ID %s already exists", runner.ID)
			return
		}
	}
	sm.runners[runner.ID] = runnerState
	sm.eventBus.Publish(events.Event{Type: tt.EventRunnerCreated, Payload: &runner})
}

// GetRunnerState returns copy of RunnerState form it's state.
// RunnerState should be read only for any other part of application to avoid accidental modifications
func (sm *StateManager) GetRunnerState(id utils.UID) (tt.RunnerState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	runner, exists := sm.runners[id]
	if !exists {
		return tt.RunnerState{}, false
	}
	return *runner, exists
}

func (sm *StateManager) GetActiveRunners() []tt.Runner {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	activeRunners := make([]tt.Runner, 0, len(sm.activeRunners))
	for _, runner := range sm.activeRunners {
		activeRunners = append(activeRunners, *runner.Runner)
	}
	return activeRunners
}

func (sm *StateManager) ListRunners() []tt.RunnerStateList {
	return sm.createRunnerStateListResponse(sm.runners)
}

func (sm *StateManager) ListActiveRunners() []tt.RunnerStateList {
	return sm.createRunnerStateListResponse(sm.activeRunners)
}

func (sm *StateManager) createRunnerStateListResponse(runners map[utils.UID]*tt.RunnerState) []tt.RunnerStateList {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	runnersStateList := make([]tt.RunnerStateList, 0, len(runners))

	for _, runner := range runners {
		runnersStateList = append(runnersStateList, tt.RunnerStateList{
			ID:            string(runner.Runner.ID),
			Name:          runner.Runner.Config.Name,
			GhaRunnerName: runner.Runner.GhaRunnerName,
			Status:        string(runner.Status),
			ErrorMessage:  runner.ErrorMessage,
		})
	}
	return runnersStateList
}

func (sm *StateManager) StopRunner(id utils.UID) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if runner, exists := sm.runners[id]; exists {
		sm.eventBus.Publish(events.Event{Type: tt.EventRunnerStopRequest, Payload: runner.Runner})
		return exists
	}
	return false
}

// UpdateRunnerStatus updates the status of a runner.
func (sm *StateManager) UpdateRunnerStatus(id utils.UID, status tt.RunnerStatus, errorMessage ...string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if runner, exists := sm.runners[id]; exists {
		// Runner changed status to running. Adding to activeRunners.
		if status == tt.Running {
			sm.activeRunners[id] = runner
			runner.StartedAt = time.Now()
		}
		if (runner.Status == tt.Running || runner.Status == tt.Stopping) && status != tt.Stopping {
			delete(sm.activeRunners, id)
		}
		if status == tt.Failed || status == tt.Finished || status == tt.Stopped {
			runner.EndedAt = time.Now()
		}

		runner.Status = status

		if len(errorMessage) > 0 {
			runner.ErrorMessage = strings.Join(errorMessage, ", ")
		}

		sm.eventBus.Publish(events.Event{Type: tt.EventRunnerStatusUpdated, Payload: runner.Runner})
	}
}

func (sm *StateManager) UpdateRunnersHealth(id utils.UID, healthResult probe.Result, probeType pt.ProbeType) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if runner, exists := sm.runners[id]; exists {
		switch probeType {
		case pt.Liveness:
			runner.Health.Liveness = healthResult
		case pt.Startup:
			runner.Health.Startup = healthResult
		}
		return exists
	}
	return false
}
