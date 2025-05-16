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
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

func MethodNotAllowedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		response := SimpleResponse{
			Status:  http.StatusMethodNotAllowed,
			Message: "The method not allowed.",
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}
}

func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		response := SimpleResponse{
			Status:  http.StatusNotFound,
			Message: "Page not found.",
		}

		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}
}

func SimpleHealthcheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := SimpleResponse{
		Status:  http.StatusOK,
		Message: "Healthy",
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

func SetupGenericHandlers(r *mux.Router) {
	// Define a custom handler for unsupported methods
	r.MethodNotAllowedHandler = MethodNotAllowedHandler()
	// Define a custom handler for 404 responses
	r.NotFoundHandler = NotFoundHandler()
}
