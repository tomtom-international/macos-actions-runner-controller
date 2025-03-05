package results

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/results"
	pt "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/events"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"sync"
)

// Prober results manager implementation.
type runnerProbeResultManager struct {
	mu        sync.RWMutex
	probeType pt.ProbeType
	// Event Bus to send health_check events
	eventBus *events.EventBus
}

var _ results.Manager = &runnerProbeResultManager{}

func NewManager(probeType pt.ProbeType, eventBus *events.EventBus) results.Manager {
	return &runnerProbeResultManager{
		probeType: probeType,
		eventBus:  eventBus,
	}
}

func (m *runnerProbeResultManager) SetResult(runnerID utils.UID, result probe.Result) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	payload := tt.HealthCheckEventPayload{
		RunnerID:     runnerID,
		HealthResult: result,
	}
	switch m.probeType {
	case pt.Liveness:
		m.eventBus.Publish(events.Event{Type: tt.EventRunnerLivenessHealthCheck, Payload: payload})
	case pt.Startup:
		m.eventBus.Publish(events.Event{Type: tt.EventRunnerStartupHealthCheck, Payload: payload})
	}

}
