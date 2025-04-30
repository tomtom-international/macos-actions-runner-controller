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

	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter"
)

func ListRunnerWorkersHandler(t *tarter.Tarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workers := t.ListRunnerWorkers()
		w.Header().Set("Content-Type", "application/json")
		resp, err := json.Marshal(workers)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal runner workers: %s", err.Error()),
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

func GetRunnerWorkersCountHandler(t *tarter.Tarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := t.GetWorkersCount()
		w.Header().Set("Content-Type", "application/json")
		resp := []byte(fmt.Sprintf("{\"count\": %d}", c))
		_, err := w.Write(resp)
		if err != nil {
			return
		}
	}
}
