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

package runners

import (
	"context"
	"encoding/json"
	"path"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type Manager struct {
	etcdClient              *etcd.EtcdClient
	runnersWatcher          *runnersWatcher
	etcdKeyPrefix           string
	etcdRunnerFinishedLease int
	runnersWatcherLock      sync.RWMutex
}

func NewManager(etcdClient *etcd.EtcdClient, etcdKeyPrefix string, etcdRunnerFinishedLease int) *Manager {
	return &Manager{
		etcdClient:              etcdClient,
		etcdKeyPrefix:           etcdKeyPrefix,
		etcdRunnerFinishedLease: etcdRunnerFinishedLease,
	}
}

// SaveRunner runner to Etcd
func (m *Manager) SaveRunner(runner *types.Runner) error {
	runner.Condition.LastTransitionTime = time.Now()
	rawRunner, err := json.Marshal(runner)
	if err != nil {
		logger.Errorf("Saving runner %v failed to marshal, error: %v", runner.ID, err)
		return err
	}
	key := path.Join("/", m.etcdKeyPrefix, string(runner.ID))
	err = m.etcdClient.Put(key, string(rawRunner))
	if err != nil {
		logger.Errorf("Saving runner %v failed, error: %v", runner.ID, err)
		return err
	}
	logger.Debugf("Runner %s saved to etcd", runner.ID)
	return nil
}

func (m *Manager) SaveRunnerWithLease(runner *types.Runner) error {
	leaseID, err := m.etcdClient.GrantLease(m.etcdRunnerFinishedLease)
	if err != nil {
		logger.Errorf("Saving runner %s failed to grant lease, error: %s", runner.ID, err.Error())
		return err
	}
	runner.Condition.LastTransitionTime = time.Now()
	rawRunner, err := json.Marshal(runner)
	if err != nil {
		logger.Errorf("Saving runner %s failed to marshal, error: %s", runner.ID, err.Error())
		return err
	}
	key := path.Join("/", m.etcdKeyPrefix, string(runner.ID))
	err = m.etcdClient.PutWithLease(key, string(rawRunner), leaseID)
	if err != nil {
		logger.Errorf("Saving runner %s failed, error: %s", runner.ID, err.Error())
		return err
	}
	logger.Debugf("Runner %s saved to etcd with lease", runner.ID)
	return nil
}

// GetRunnerByID gets runner from Etcd by ID
func (m *Manager) GetRunnerByID(runnerID utils.UID) (*types.Runner, error) {
	key := path.Join("/", m.etcdKeyPrefix, string(runnerID))
	rawRunner, err := m.etcdClient.Get(key)
	if err != nil {
		logger.Errorf("GetRunnerById failed to get runner %v, error: %v", runnerID, err)
		return nil, err
	}
	if rawRunner == "" {
		return nil, nil
	}
	var runner types.Runner
	err = json.Unmarshal([]byte(rawRunner), &runner)
	if err != nil {
		logger.Errorf("GetRunnerById failed to unmarshal runner %v, error: %v", runnerID, err)
		return nil, err
	}
	return &runner, nil
}

// GetRunners gets all runners from Etcd
func (m *Manager) GetRunners() ([]types.Runner, error) {
	key := path.Join("/", m.etcdKeyPrefix)
	rawRunners, err := m.etcdClient.GetByPrefix(key)
	if err != nil {
		logger.Errorf("GetRunners failed to get Runners from etcd, error: %s", err)
		return nil, err
	}
	runners := make([]types.Runner, 0, len(rawRunners))
	for _, val := range rawRunners {
		var runner types.Runner
		if err := json.Unmarshal([]byte(val), &runner); err != nil {
			logger.Errorf("GetRunners failed to unmarshal Runner, error: %s", err)
			return nil, err
		}
		runners = append(runners, runner)
	}
	return runners, nil
}

func (m *Manager) GetRunnerByCreateRequestID(requestID string) (*types.Runner, error) {
	key := path.Join("/", m.etcdKeyPrefix)
	rawRunners, err := m.etcdClient.GetByPrefix(key)
	if err != nil {
		logger.Errorf("GetRunnerByCreateRequestID failed to get Runners from etcd, error: %s", err)
		return nil, err
	}
	for _, val := range rawRunners {
		var runner types.Runner
		if err := json.Unmarshal([]byte(val), &runner); err != nil {
			logger.Errorf("GetRunnerByCreateRequestID failed to unmarshal Runner, error: %s", err)
			return nil, err
		}
		if runner.Condition.CreateRequestID == requestID {
			return &runner, nil
		}
	}
	return nil, nil
}

// AddRunnerWatcher watches for runner changes in Etcd
// Should be called only once for read operations
func (m *Manager) AddRunnerWatcher(conn *websocket.Conn, notificationChan chan types.WatcherRunnersUpdate) {
	m.runnersWatcherLock.Lock()
	defer m.runnersWatcherLock.Unlock()

	if m.runnersWatcher == nil {
		logger.Debugf("Creating new runners watcher")
		watchCtx, cancel := context.WithCancel(context.Background())
		m.runnersWatcher = newRunnersWatcher(
			watchCtx,
			cancel,
			m.etcdClient,
			path.Join("/", m.etcdKeyPrefix),
			m,
		)
	}

	m.runnersWatcher.AddSubscriber(conn, notificationChan)

	// Start watching if not already watching
	if !m.runnersWatcher.watching {
		logger.Debugf("Starting runners watcher")
		go m.runnersWatcher.watch()
		m.runnersWatcher.watching = true
	}
}

func (m *Manager) RemoveRunnersWatcherHandler(conn *websocket.Conn) {
	m.runnersWatcherLock.Lock()
	defer m.runnersWatcherLock.Unlock()

	if m.runnersWatcher != nil {
		handlersCount := m.runnersWatcher.RemoveSubscriber(conn)
		if handlersCount == 0 {
			m.runnersWatcher.cancelFn()
			m.runnersWatcher = nil
		}
	}
}
