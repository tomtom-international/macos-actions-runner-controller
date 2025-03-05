package prober

import (
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/results"
	pt "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/state"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"sync"
)

type ProberManager struct {
	// Map of active workers for probes
	workers map[probeTargetKey]*worker
	// Lock for accessing & mutating workers
	workerLock sync.RWMutex

	//livenessManager manages the results of liveness probes
	livenessManager results.Manager

	//startupManager manages the results of startup probes
	startupManager results.Manager

	// prober executes the probe actions.
	prober *prober

	stateManager *state.StateManager
}

func NewManager(
	githubClient *ghclient.Client,
	stateManager *state.StateManager,
	livenessManager results.Manager,
	startupManager results.Manager) *ProberManager {

	prober := newProber(githubClient)
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
			logger.Errorf("Startup probe already exists for target: id - %d, name - %s", target.ID, target.Name)
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
			logger.Errorf("Liveness probe already exists for target: id - %d, name - %s", target.ID, target.Name)
			return
		}
		w := newWorker(m, pt.Liveness, target, m.livenessManager)
		m.workers[targetKey] = w
		go w.run()
	}
}

func (m *ProberManager) getWorker(targetID utils.UID, targetName string, probeType pt.ProbeType) (*worker, bool) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	worker, ok := m.workers[probeTargetKey{targetID, targetName, probeType}]
	return worker, ok
}

// Called by the worker after exiting.
func (m *ProberManager) removeWorker(targetID utils.UID, targetName string, probeType pt.ProbeType) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, probeTargetKey{targetID, targetName, probeType})
}
