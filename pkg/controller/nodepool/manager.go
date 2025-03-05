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

package nodepool

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"path"
	"sync"
)

type Manager struct {
	etcdClient              *etcd.EtcdClient
	etcdKeyPrefix           string
	etcdNodeDeregisterLease int
	heartbeatWatchers       map[utils.UID]*heartbeatWatcher
	heartbeatWatcherLock    sync.RWMutex
	nodesWatcher            *nodesWatcher
	nodesWatcherLock        sync.RWMutex
}

func NewManager(etcdClient *etcd.EtcdClient, etcdKeyPrefix string, etcdNodeDeregisterLease int) *Manager {
	return &Manager{
		etcdClient:              etcdClient,
		etcdKeyPrefix:           etcdKeyPrefix,
		etcdNodeDeregisterLease: etcdNodeDeregisterLease,
		heartbeatWatchers:       make(map[utils.UID]*heartbeatWatcher),
	}
}

func (m *Manager) SaveNode(node types.Node) error {
	rawNode, err := json.Marshal(node)
	if err != nil {
		logger.Errorf("SaveNode failed to marshal %v, error: %v", node.ID, err)
		return err
	}
	key := path.Join("/", m.etcdKeyPrefix, string(node.ID))
	err = m.etcdClient.Put(key, string(rawNode))
	if err != nil {
		logger.Errorf("SaveNode failed to save node %v, error: %v", node.ID, err)
		return err
	}
	return nil
}

func (m *Manager) SaveNodeWithLease(node types.Node) error {
	leaseID, err := m.etcdClient.GrantLease(m.etcdNodeDeregisterLease)
	if err != nil {
		logger.Errorf("SaveNodeWithLease failed to grant lease for node %v, error: %v", node.ID, err)
		return err
	}

	rawNode, err := json.Marshal(node)
	if err != nil {
		logger.Errorf("SaveNodeWithLease failed to marshal %v, error: %v", node.ID, err)
		return err
	}
	key := path.Join("/", m.etcdKeyPrefix, string(node.ID))
	err = m.etcdClient.PutWithLease(key, string(rawNode), leaseID)
	if err != nil {
		logger.Errorf("SaveNodeWithLease failed to save node %v, error: %v", node.ID, err)
		return err
	}
	return nil
}

func (m *Manager) AddNodeHeartbeatWatcher(nodeID utils.UID) {
	logger.Debugf("Adding heartbeat watcher for node %s", nodeID)
	watcher := newHeartbeatWatcher(nodeID, m.etcdClient, m)

	if _, ok := m.heartbeatWatchers[nodeID]; ok {
		logger.Warnf("Heardbeat watcher for node %s already exists", nodeID)
		return
	}

	m.heartbeatWatcherLock.Lock()
	m.heartbeatWatchers[nodeID] = watcher
	m.heartbeatWatcherLock.Unlock()

	go watcher.run()
}

func (m *Manager) IsHeartbeatWatching(nodeID utils.UID) bool {
	m.heartbeatWatcherLock.Lock()
	defer m.heartbeatWatcherLock.Unlock()
	_, ok := m.heartbeatWatchers[nodeID]
	return ok
}

// removeNodeHeartbeatWatcher removes the node heartbeat watcher from the manager
// called by the watcher when it stops with defer
func (m *Manager) removeNodeHeartbeatWatcher(nodeID utils.UID) {
	m.heartbeatWatcherLock.Lock()
	defer m.heartbeatWatcherLock.Unlock()
	delete(m.heartbeatWatchers, nodeID)
}

func (m *Manager) GetNodeById(nodeID utils.UID) (*types.Node, error) {
	key := path.Join("/", m.etcdKeyPrefix, string(nodeID))
	rawNode, err := m.etcdClient.Get(key)
	if err != nil {
		logger.Errorf("GetNodeById failed to get Node %s from etcd, error: %s", nodeID, err)
		return nil, err
	}
	if rawNode == "" {
		return nil, nil
	}
	var node types.Node
	err = json.Unmarshal([]byte(rawNode), &node)
	if err != nil {
		logger.Errorf("GetNodeById failed to unmarshal Node %s, error: %s", nodeID, err)
		return nil, err
	}

	return &node, nil
}

func (m *Manager) GetNodeByName(nodeName string) (*types.Node, error) {
	key := path.Join("/", m.etcdKeyPrefix)
	rawNodePool, err := m.etcdClient.GetByPrefix(key)
	if err != nil {
		logger.Errorf("GetNodeByName failed to get Node %s from etcd, error: %s", nodeName, err)
		return nil, err
	}
	for _, val := range rawNodePool {
		var node types.Node
		if err := json.Unmarshal([]byte(val), &node); err != nil {
			logger.Errorf("GetNodeByName failed to unmarshal Node %s, error: %s", nodeName, err)
			return nil, err
		}
		if node.Name == nodeName {
			return &node, nil
		}
	}
	return nil, nil
}

func (m *Manager) GetNodePool() ([]types.Node, error) {
	key := path.Join("/", m.etcdKeyPrefix)
	rawNodePool, err := m.etcdClient.GetByPrefix(key)
	if err != nil {
		logger.Errorf("GetNodePool failed to get data from etcd, error: %s", err)
		return nil, err
	}
	nodePool := make([]types.Node, 0, len(rawNodePool))
	for _, val := range rawNodePool {
		var node types.Node
		if err := json.Unmarshal([]byte(val), &node); err != nil {
			logger.Errorf("GetNodePool failed to unmarshal NodePool, error: %s", err)
			return nil, err
		}
		nodePool = append(nodePool, node)
	}

	return nodePool, nil
}

// GetHeartbeatWatcherCount returns the number of active heartbeat watchers
func (m *Manager) GetHeartbeatWatcherCount() int {
	m.heartbeatWatcherLock.RLock()
	defer m.heartbeatWatcherLock.RUnlock()
	return len(m.heartbeatWatchers)
}

func (m *Manager) AddNodesWatcher(conn *websocket.Conn, notificationChan chan types.WatcherNodesUpdate) {
	m.nodesWatcherLock.Lock()
	defer m.nodesWatcherLock.Unlock()

	if m.nodesWatcher == nil {
		logger.Debugf("Creating new nodes watcher")
		watchCtx, cancel := context.WithCancel(context.Background())
		m.nodesWatcher = newNodesWatcher(
			watchCtx,
			cancel,
			m.etcdClient,
			path.Join("/", m.etcdKeyPrefix),
			m,
		)
	}

	m.nodesWatcher.addSubscriber(conn, notificationChan)

	// Start watching if not already watching
	if !m.nodesWatcher.watching {
		logger.Debugf("Starting nodes watcher")
		go m.nodesWatcher.watch()
		m.nodesWatcher.watching = true
	}
}

func (m *Manager) RemoveNodesWatcherHandler(conn *websocket.Conn) {
	m.nodesWatcherLock.Lock()
	defer m.nodesWatcherLock.Unlock()

	if m.nodesWatcher != nil {
		handlersCount := m.nodesWatcher.removeSubscriber(conn)
		if handlersCount == 0 {
			m.nodesWatcher.cancelFn()
			m.nodesWatcher = nil
		}
	}
}
