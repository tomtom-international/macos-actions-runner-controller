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

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/state"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

func ListRunnersHandler(sm *state.StateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := r.URL.Query().Get("status")

		var resp []byte
		var err error

		if status == "all" {
			resp, err = json.Marshal(sm.ListRunners())
		} else {
			resp, err = json.Marshal(sm.ListActiveRunners())
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal runners from state: %v", err),
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		_, err = w.Write(resp)
		if err != nil {
			return
		}
	}
}

func GetRunnerHandler(sm *state.StateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		runnerID := utils.UID(vars["id"])
		if runnerID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Runner id is required.",
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		runner, exists := sm.GetRunnerState(runnerID)
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			response := core.SimpleResponse{
				Status:  http.StatusNotFound,
				Message: fmt.Sprintf("The runner with id %v not found.", runnerID),
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		resp, err := json.Marshal(runner)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal runner from state: %v", err),
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		_, err = w.Write(resp)
		if err != nil {
			return
		}
	}
}

func StopRunnerHandler(sm *state.StateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		runnerID := utils.UID(vars["id"])
		if runnerID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Runner id is required.",
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		exists := sm.StopRunner(runnerID)
		if !exists {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: fmt.Sprintf("The runner with id %v not found.", runnerID),
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}

		response := core.SimpleResponse{
			Message: fmt.Sprintf("Runner with id %v is stopping", runnerID),
		}
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}
}
