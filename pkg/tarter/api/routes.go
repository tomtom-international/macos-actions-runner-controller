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
	"github.com/gorilla/mux"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/api/handlers"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/state"
)

func SetupRoutes(r *mux.Router, sm *state.StateManager) {
	r.HandleFunc("/runners", handlers.ListRunnersHandler(sm)).Methods("GET")
	r.HandleFunc("/runners/{id}", handlers.GetRunnerHandler(sm)).Methods("GET")
	r.HandleFunc("/runners/{id}/stop", handlers.StopRunnerHandler(sm)).Methods("PUT")
}

func SetupSysRoutes(r *mux.Router, t *tarter.Tarter) {
	r.HandleFunc("/sys/workers", handlers.ListRunnerWorkersHandler(t)).Methods("GET")
	r.HandleFunc("/sys/workers-count", handlers.GetRunnerWorkersCountHandler(t)).Methods("GET")
}

func SetupStandaloneTarterRoutes(r *mux.Router, t *tarter.Tarter) {
	r.HandleFunc("/sys/standalone/config", handlers.GetStandaloneConfigHandler(t)).Methods("GET")
	r.HandleFunc("/sys/standalone/restart", handlers.RestartStandaloneRunnersHandler(t)).Methods("PUT")
}
