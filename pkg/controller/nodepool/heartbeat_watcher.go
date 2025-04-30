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
	"fmt"
	"path"
	"time"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	watchTimeoutSeconds = 40
)

type heartbeatWatcher struct {
	ctx         context.Context
	etcdClient  *etcd.EtcdClient
	ctxCancel   context.CancelFunc
	nodeManager *Manager
	nodeID      utils.UID
	watchKey    string
	pause       bool
}

func newHeartbeatWatcher(
	nodeID utils.UID,
	etcdClient *etcd.EtcdClient,
	nodeManager *Manager,
) *heartbeatWatcher {

	ctx, cancel := context.WithCancel(context.Background())
	key := path.Join("/", nodeManager.etcdKeyPrefix, string(nodeID))

	return &heartbeatWatcher{
		nodeID:      nodeID,
		watchKey:    key,
		etcdClient:  etcdClient,
		ctx:         ctx,
		ctxCancel:   cancel,
		nodeManager: nodeManager,
	}
}

// TODO: consider to rely on heartbeat request instead of etcd watcher because it triggers on every node change
//
//gocyclo:ignore
func (w *heartbeatWatcher) run() {
	defer func() {
		logger.Debugf("Stopping heartbeat watcher for node %s", w.nodeID)
		w.ctxCancel()
		w.nodeManager.removeNodeHeartbeatWatcher(w.nodeID)
	}()

	w.pause = false

	for {
		watchChan := w.etcdClient.Watch(w.ctx, w.watchKey)

		for {
			select {
			case <-w.ctx.Done():
				return
			case watchResp, ok := <-watchChan:
				if !ok {
					w.etcdClient.ErrChan <- fmt.Errorf("watch channel closed for key %s, attempting to reconnect", w.watchKey)
					time.Sleep(time.Second)
					break
				}

				if watchResp.Err() != nil {
					w.etcdClient.ErrChan <- fmt.Errorf("watch error for key %s: %w", w.watchKey, watchResp.Err())
					break
				}
				for _, ev := range watchResp.Events {
					if ev.Type == clientv3.EventTypePut {
						var node types.Node
						err := json.Unmarshal(ev.Kv.Value, &node)
						if err != nil {
							logger.Errorf("Heartbeat Watcher failed to unmarshal node %s: %v", w.nodeID, err)
							continue
						}
						if node.Status.Condition.LastHeartbeatTime.Add(3 * time.Second).After(time.Now()) {
							w.pause = false
						} else {
							continue
						}
					}
					if ev.Type == clientv3.EventTypeDelete {
						logger.Debugf("Received delete event for node %s.", w.nodeID)
						return
					}
				}
			case <-time.After(watchTimeoutSeconds * time.Second):
				if w.pause {
					continue
				}
				// TODO: fix concurrent watchers on every replicas
				logger.Debugf("Heartbeat watcher for node %s timed out.", w.nodeID)
				node, err := w.nodeManager.GetNodeByID(w.nodeID)
				if err == nil && time.Since(node.Status.Condition.LastHeartbeatTime) > watchTimeoutSeconds*time.Second {
					if node.Status.Condition.Status == types.Unknown || node.Status.Condition.Status == types.Deregistered {
						continue
					}

					node.Status.Condition.Healthy = false
					node.Status.Condition.Message = "Heartbeat timeout"
					node.Status.Condition.LastTransitionTime = time.Now()

					if node.Status.Condition.Status != types.Disabled {
						node.Status.Condition.Status = types.Unknown
					}

					err = w.nodeManager.SaveNode(*node)
					if err != nil {
						logger.Errorf("Error updating node %s Heartbeat timeout status: %v", w.nodeID, err)
					}
					w.pause = true
				}
			}
		}
	}
}
