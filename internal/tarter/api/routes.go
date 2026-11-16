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

package api

import (
	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/api/handlers"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
)

func NewRouter() *core.Router {
	return core.NewRouter()
}

func SetupRoutes(r *core.Router, sm handlers.RunnerStore) {
	r.HandleFunc("GET /runners", handlers.ListRunnersHandler(sm))
	r.HandleFunc("GET /runners/{id}", handlers.GetRunnerHandler(sm))
	r.HandleFunc("PUT /runners/{id}/stop", handlers.StopRunnerHandler(sm))
}

func SetupSysRoutes(r *core.Router, t handlers.Workers) {
	r.HandleFunc("GET /sys/workers", handlers.ListRunnerWorkersHandler(t))
	r.HandleFunc("GET /sys/workers-count", handlers.GetRunnerWorkersCountHandler(t))
}

func SetupStandaloneTarterRoutes(r *core.Router, t handlers.Standalone) {
	r.HandleFunc("GET /sys/standalone/config", handlers.GetStandaloneConfigHandler(t))
	r.HandleFunc("PUT /sys/standalone/restart", handlers.RestartStandaloneRunnersHandler(t))
}
