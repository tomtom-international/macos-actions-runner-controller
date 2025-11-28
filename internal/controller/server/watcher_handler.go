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
	"strings"

	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

// TODO: remove debug logs
func WatcherHandler(controller ControllerInterface, allowedOrigins string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origins := parseOrigins(allowedOrigins)

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")

				if origin == "" {
					logger.Debugf("WebSocket connection with no Origin header")
					return true
				}

				allowed := isOriginAllowed(origin, origins)
				if !allowed {
					logger.Warnf("WebSocket connection rejected from origin: %s", origin)
				}
				return allowed
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

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

// parseOrigins parses a comma-separated list of allowed origins
func parseOrigins(originsStr string) []string {
	if originsStr == "" || originsStr == "*" {
		return []string{"*"}
	}

	origins := strings.Split(originsStr, ",")
	result := make([]string, 0, len(origins))
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// isOriginAllowed checks if the given origin is in the allowed list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		return true
	}

	origin = strings.TrimSuffix(origin, "/")

	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSuffix(allowed, "/")

		if origin == allowed {
			return true
		}

		if strings.HasPrefix(allowed, "*.") {
			domain := allowed[2:]
			if strings.HasSuffix(origin, domain) ||
				strings.Contains(origin, "://") && strings.HasSuffix(strings.Split(origin, "://")[1], domain) {
				return true
			}
		}
	}

	return false
}
