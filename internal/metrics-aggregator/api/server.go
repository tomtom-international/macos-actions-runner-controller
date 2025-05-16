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
	"fmt"
	"net"
	"net/http"
	"time"

	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

type MetricsAggregatorService interface {
	CollectRunnersMetrics() (string, error)
}

func (s *Server) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /metrics", s.handleGetMetrics)
	mux.HandleFunc("GET /health", core.SimpleHealthcheckHandler)
}

// Server represents the HTTP server
type Server struct {
	maService MetricsAggregatorService
	server    *http.Server
}

// NewServer creates a new HTTP server and sets up the routes
func NewServer(maService MetricsAggregatorService, address net.IP, port string) *Server {
	mux := http.NewServeMux()

	server := &Server{
		maService: maService,
		server: &http.Server{
			Addr:           fmt.Sprintf("%s:%s", address, port),
			Handler:        mux,
			ReadTimeout:    2 * 60 * time.Second,
			WriteTimeout:   1 * 60 * time.Second,
			IdleTimeout:    120 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}

	server.setupRoutes(mux)

	return server
}

// ListenAndServe starts the HTTP server and listens for incoming requests
func (s *Server) ListenAndServe() error {
	logger.Infof("Starting http server on address %s", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.maService.CollectRunnersMetrics()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := core.SimpleResponse{
			Status:  http.StatusInternalServerError,
			Message: fmt.Sprintf("Error collecting metrics: %v", err),
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, err = w.Write([]byte(metrics))
	if err != nil {
		return
	}
}
