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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type runnersWatcher struct {
	ctx         context.Context
	etcdClient  *etcd.EtcdClient
	subscribers map[*websocket.Conn]chan types.WatcherRunnersUpdate
	cancelFn    context.CancelFunc
	manager     *Manager
	key         string
	mu          sync.RWMutex
	watching    bool
}

func newRunnersWatcher(
	ctx context.Context,
	cancel context.CancelFunc,
	etcdClient *etcd.EtcdClient,
	key string,
	manager *Manager,
) *runnersWatcher {

	return &runnersWatcher{
		subscribers: make(map[*websocket.Conn]chan types.WatcherRunnersUpdate),
		ctx:         ctx,
		cancelFn:    cancel,
		etcdClient:  etcdClient,
		key:         key,
		manager:     manager,
	}
}

func (w *runnersWatcher) watch() {
	logger.Debugf("Runners watcher for key %s started", w.key)
	for {
		watchChan := w.etcdClient.WatchWithPrefix(w.ctx, w.key)
		for {
			select {
			case <-w.ctx.Done():
				logger.Debugf("Runners watcher stopped")
				return
			case watchResp, ok := <-watchChan:
				if !ok {
					w.etcdClient.ErrChan <- fmt.Errorf("runners watcher channel closed, attempting to reconnect")
					time.Sleep(time.Second)
					break
				}

				if watchResp.Err() != nil {
					w.etcdClient.ErrChan <- fmt.Errorf("runners watcher error for key %s: %v", w.key, watchResp.Err().Error())
					break
				}

				runnersUpdate := types.WatcherRunnersUpdate{
					UpdatedRunners: make([]types.Runner, 0),
					DeletedRunners: make([]types.Runner, 0),
				}
				for _, event := range watchResp.Events {
					if event.Type == clientv3.EventTypePut {
						var runner types.Runner
						err := json.Unmarshal(event.Kv.Value, &runner)
						if err != nil {
							logger.Debugf("Raw event value: %+v", event.Kv)
							w.etcdClient.ErrChan <- fmt.Errorf("failed to unmarshal runners watcher Put event value. Error: %s", err.Error())
							continue
						}
						runnersUpdate.UpdatedRunners = append(runnersUpdate.UpdatedRunners, runner)
					}
					if event.Type == clientv3.EventTypeDelete {
						// Remove key prefix and extract runner ID
						id := utils.UID(strings.Split(string(event.Kv.Key), "/")[2])
						runnersUpdate.DeletedRunners = append(runnersUpdate.DeletedRunners, types.Runner{ID: id})
					}
				}

				// Notify all subscribers
				w.mu.RLock()
				for _, notificationChan := range w.subscribers {
					notificationChan <- runnersUpdate
				}
				w.mu.RUnlock()
			}
		}
	}
}

func (w *runnersWatcher) AddSubscriber(conn *websocket.Conn, notificationChan chan types.WatcherRunnersUpdate) {
	w.mu.Lock()
	defer w.mu.Unlock()
	logger.Debugf("Adding runners watcher subscriber")
	w.subscribers[conn] = notificationChan
}

func (w *runnersWatcher) RemoveSubscriber(conn *websocket.Conn) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	logger.Debugf("Removing runners watcher subscriber")
	delete(w.subscribers, conn)
	subscriberCount := len(w.subscribers)
	logger.Debugf("Subscribers Count: %d", subscriberCount)

	return subscriberCount
}

// GetSubscriberCount returns the total number of Subscribers registered for the watcher
func (w *runnersWatcher) GetSubscriberCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return len(w.subscribers)
}
