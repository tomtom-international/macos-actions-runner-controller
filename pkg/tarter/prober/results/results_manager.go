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
