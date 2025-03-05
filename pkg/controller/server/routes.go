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
	"github.com/gorilla/mux"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
)

func SetupAuditRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthCheckHandler()).Methods("GET")
}

func SetupNodePoolRoutes(r *mux.Router, c ControllerInterface) {
	r.HandleFunc("/nodes", GetNodeListHandler(c)).Methods("GET", "OPTIONS")
	r.HandleFunc("/nodes/register", RegisterNodeHandler(c)).Methods("POST")
	r.HandleFunc("/nodes/{node_uid}", GetNodeHandler(c)).Methods("GET")
	r.HandleFunc("/nodes/{node_uid}/status", NodeHeartbeatHandler(c)).Methods("PUT")
	r.HandleFunc("/nodes/{node_uid}/deregister", UpdateNodeStatusHandler(c, types.Deregistered)).Methods("PUT")
	r.HandleFunc("/nodes/{node_uid}/enable", UpdateNodeStatusHandler(c, types.NotReady)).Methods("PUT")
	r.HandleFunc("/nodes/{node_uid}/disable", UpdateNodeStatusHandler(c, types.Disabled)).Methods("PUT")
}

func SetupRunnersRoutes(r *mux.Router, c ControllerInterface) {
	r.HandleFunc("/runners", GetRunnerListHandler(c)).Methods("GET", "OPTIONS")
	r.HandleFunc("/runners/{runner_uid}", GetRunnerHandler(c)).Methods("GET")
	r.HandleFunc("/runners/{runner_uid}/status", GetRunnerStatusUpdateHandler(c)).Methods("PUT")
}

func SetupWatcherRoutes(r *mux.Router, c ControllerInterface) {
	r.HandleFunc("/ws/watch", WatcherHandler(c)).Methods("GET")
}
