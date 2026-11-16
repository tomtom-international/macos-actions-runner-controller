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
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
)

func SetupAuditRoutes(r *core.Router) {
	r.HandleFunc("GET /api/v1/health", HealthCheckHandler())
}

func SetupNodePoolRoutes(r *core.Router, c ControllerInterface, origins string) {
	r.HandleFunc("GET /api/v1/nodes", GetNodeListHandler(c, origins))
	r.HandleFunc("OPTIONS /api/v1/nodes", GetNodeListHandler(c, origins))
	r.HandleFunc("POST /api/v1/nodes/register", RegisterNodeHandler(c))
	r.HandleFunc("GET /api/v1/nodes/{node_uid}", GetNodeHandler(c))
	r.HandleFunc("PUT /api/v1/nodes/{node_uid}/status", NodeHeartbeatHandler(c))
	r.HandleFunc("PUT /api/v1/nodes/{node_uid}/deregister", UpdateNodeStatusHandler(c, types.Deregistered))
	r.HandleFunc("PUT /api/v1/nodes/{node_uid}/enable", UpdateNodeStatusHandler(c, types.NotReady))
	r.HandleFunc("PUT /api/v1/nodes/{node_uid}/disable", UpdateNodeStatusHandler(c, types.Disabled))
}

func SetupRunnersRoutes(r *core.Router, c ControllerInterface, origins string) {
	r.HandleFunc("GET /api/v1/runners", GetRunnerListHandler(c, origins))
	r.HandleFunc("OPTIONS /api/v1/runners", GetRunnerListHandler(c, origins))
	r.HandleFunc("GET /api/v1/runners/{runner_uid}", GetRunnerHandler(c))
	r.HandleFunc("PUT /api/v1/runners/{runner_uid}/status", GetRunnerStatusUpdateHandler(c))
}

func SetupWatcherRoutes(r *core.Router, c ControllerInterface, origins string) {
	r.HandleFunc("GET /ws/watch", WatcherHandler(c, origins))
}
