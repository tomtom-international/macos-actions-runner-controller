/*
 * Copyright 2025 TomTom N.V.
 * Copyright 2015 The Kubernetes Authors.
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

package prober

import (
	"sync"

	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/prober/results"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/state"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	tartclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/tart"
	pt "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/prober"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type ProberManager struct {
	livenessManager results.Manager
	startupManager  results.Manager
	workers         map[probeTargetKey]*worker
	prober          *prober
	stateManager    *state.StateManager
	workerLock      sync.RWMutex
}

func NewManager(
	githubClient *ghclient.Client,
	tartClient *tartclient.Client,
	stateManager *state.StateManager,
	livenessManager results.Manager,
	startupManager results.Manager) *ProberManager {

	prober := newProber(githubClient, tartClient)
	return &ProberManager{
		stateManager:    stateManager,
		prober:          prober,
		livenessManager: livenessManager,
		startupManager:  startupManager,
		workers:         make(map[probeTargetKey]*worker),
	}
}

type probeTargetKey struct {
	targetID   utils.UID
	targetName string
	probeType  pt.ProbeType
}

func (m *ProberManager) AddProber(target *pt.ProbeTarget) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	targetKey := probeTargetKey{targetID: target.ID}
	targetKey.targetName = target.Name

	if target.StartupProbe != nil {
		targetKey.probeType = pt.Startup
		logger.Debugf("Starting %v probe for target %v ...", targetKey.probeType.String(), targetKey.targetID)
		if _, ok := m.workers[targetKey]; ok {
			logger.Errorf("Startup probe already exists for target: id - %s, name - %s", target.ID, target.Name)
			return
		}
		w := newWorker(m, pt.Startup, target, m.startupManager)
		m.workers[targetKey] = w
		go w.run()
	}

	if target.LivenessProbe != nil {
		targetKey.probeType = pt.Liveness
		logger.Debugf("Starting %v probe for target %v ...", targetKey.probeType.String(), targetKey.targetID)
		if _, ok := m.workers[targetKey]; ok {
			logger.Errorf("Liveness probe already exists for target: id - %s, name - %s", target.ID, target.Name)
			return
		}
		w := newWorker(m, pt.Liveness, target, m.livenessManager)
		m.workers[targetKey] = w
		go w.run()
	}
}

// Called by the worker after exiting.
func (m *ProberManager) removeWorker(targetID utils.UID, targetName string, probeType pt.ProbeType) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, probeTargetKey{targetID, targetName, probeType})
}
