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

	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
)

func GetStandaloneConfigHandler(t *tarter.Tarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, err := json.Marshal(t.GetStandaloneConfig())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal standalone config: %v", err),
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

func RestartStandaloneRunnersHandler(t *tarter.Tarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		t.RestartStandaloneRunners()

		response := core.SimpleResponse{
			Message: "Standalone config restarted",
		}
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}
}
