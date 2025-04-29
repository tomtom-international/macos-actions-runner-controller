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
	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
	"strings"
	"sync"
	"time"
)

type nodesWatcher struct {
	ctx         context.Context
	etcdClient  *etcd.EtcdClient
	subscribers map[*websocket.Conn]chan types.WatcherNodesUpdate
	cancelFn    context.CancelFunc
	manager     *Manager
	key         string
	mu          sync.RWMutex
	watching    bool
}

func newNodesWatcher(
	ctx context.Context,
	cancel context.CancelFunc,
	etcdClient *etcd.EtcdClient,
	key string,
	manager *Manager,
) *nodesWatcher {
	return &nodesWatcher{
		subscribers: make(map[*websocket.Conn]chan types.WatcherNodesUpdate),
		ctx:         ctx,
		cancelFn:    cancel,
		etcdClient:  etcdClient,
		key:         key,
		manager:     manager,
	}
}

// watch starts the watch loop for the nodes
func (w *nodesWatcher) watch() {
	logger.Debugf("Nodes watcher for key %s started", w.key)
	for {
		watchChan := w.etcdClient.WatchWithPrefix(w.ctx, w.key)

		for {
			select {
			case <-w.ctx.Done():
				logger.Debugf("Nodes watcher stopped")
				return
			case watchResp, ok := <-watchChan:
				if !ok {
					w.etcdClient.ErrChan <- fmt.Errorf("nodes watcher channel closed, attempting to reconnect")
					time.Sleep(time.Second)
					break
				}

				if watchResp.Err() != nil {
					w.etcdClient.ErrChan <- fmt.Errorf("nodes watcher error for key %s: %v", w.key, watchResp.Err().Error())
					break
				}

				nodeUpdate := types.WatcherNodesUpdate{
					UpdatedNodes: make([]types.Node, 0),
					DeletedNodes: make([]types.Node, 0),
				}
				for _, event := range watchResp.Events {
					if event.Type == clientv3.EventTypePut {
						var node types.Node
						err := json.Unmarshal(event.Kv.Value, &node)
						if err != nil {
							w.etcdClient.ErrChan <- fmt.Errorf("failed to unmarshal nodes watcher Put event value. Error: %v", err)
							continue
						}
						nodeUpdate.UpdatedNodes = append(nodeUpdate.UpdatedNodes, node)
					}
					if event.Type == clientv3.EventTypeDelete {
						// Remove key prefix and extract node ID
						id := utils.UID(strings.Split(string(event.Kv.Key), "/")[2])
						nodeUpdate.DeletedNodes = append(nodeUpdate.DeletedNodes, types.Node{ID: id})
					}
				}

				// Notify all subscribers
				w.mu.RLock()
				for _, notificationChan := range w.subscribers {
					notificationChan <- nodeUpdate
				}
				w.mu.RUnlock()
			}
		}
	}
}

func (w *nodesWatcher) addSubscriber(conn *websocket.Conn, notificationChan chan types.WatcherNodesUpdate) {
	w.mu.Lock()
	defer w.mu.Unlock()
	logger.Debugf("Adding nodes watcher subscriber")
	w.subscribers[conn] = notificationChan
}

func (w *nodesWatcher) removeSubscriber(conn *websocket.Conn) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	logger.Debugf("Removing nodes watcher subscriber")
	delete(w.subscribers, conn)
	subscriberCount := len(w.subscribers)
	logger.Debugf("Subscribers Count: %d", subscriberCount)

	return subscriberCount
}

// getSubscriberCount returns the total number of Subscribers registered for the watcher
func (w *nodesWatcher) getSubscriberCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return len(w.subscribers)
}
