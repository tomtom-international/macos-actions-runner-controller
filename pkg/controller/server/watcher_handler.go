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

package server

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

// TODO: validate origin
// TODO: remove debug logs
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WatcherHandler(controller ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Errorf("Websocket connection upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		logger.Debugf("Created new websocket connection and channel")
		nodesUpdateChan := make(chan types.WatcherNodesUpdate, 10)
		runnerUpdateChan := make(chan types.WatcherRunnersUpdate, 10)

		controller.AddNodesWatcher(conn, nodesUpdateChan)
		controller.AddRunnersWatcher(conn, runnerUpdateChan)

		for {
			select {
			case message, ok := <-nodesUpdateChan:
				if !ok {
					logger.Debugf("Watcher Handler channel closed")
					break
				}
				err = conn.WriteJSON(message)
				if err != nil {
					logger.Warnf("write: %v", err)
					controller.RemoveNodesWatcherHandler(conn)
					break
				}
			case message, ok := <-runnerUpdateChan:
				if !ok {
					logger.Debugf("Watcher Handler channel closed")
					break
				}
				err = conn.WriteJSON(message)
				if err != nil {
					logger.Warnf("write: %v", err)
					controller.RemoveRunnersWatcherHandler(conn)
					break
				}
			}
		}
	}
}
