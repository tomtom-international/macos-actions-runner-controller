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
	"strconv"
	"time"

	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/forecast"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

// ForecastService defines the interface for the forecast service
type ForecastService interface {
	GetNextScalerEvent() ([]forecast.Event, error)
	GetUpcomingScalerEvents(days int, tz *time.Location) ([]forecast.Event, error)
	IsLeader() bool
}

// Server represents the HTTP server
type Server struct {
	forecastService ForecastService
	server          *http.Server
}

func (s *Server) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/forecast", s.handleGetForecast)
	mux.HandleFunc("GET /api/leader", s.handleGetLeaderStatus)
	mux.HandleFunc("GET /health", core.SimpleHealthcheckHandler)
}

// NewServer creates a new HTTP server and sets up the routes
func NewServer(forecastService ForecastService, address net.IP, port string) *Server {
	mux := http.NewServeMux()

	server := &Server{
		forecastService: forecastService,
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

// handleGetForecast handles the GET /api/forecast endpoint
func (s *Server) handleGetForecast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var err error
	var events []forecast.Event

	if daysParam := r.URL.Query().Get("days"); daysParam != "" {
		days, err := strconv.Atoi(daysParam)
		if err != nil || days <= 0 || days > 30 {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Invalid 'days' parameter. Must be between 1 and 30.",
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}

		tz := time.UTC // Default: UTC
		if tzParam := r.URL.Query().Get("tz"); tzParam != "" {
			var err error
			tz, err = time.LoadLocation(tzParam)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				response := core.SimpleResponse{
					Status:  http.StatusBadRequest,
					Message: "Invalid 'tz' parameter. Must be a valid timezone name.",
				}
				err = json.NewEncoder(w).Encode(response)
				if err != nil {
					return
				}
				return
			}
		}
		events, err = s.forecastService.GetUpcomingScalerEvents(days, tz)
	} else {
		events, err = s.forecastService.GetNextScalerEvent()
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response := core.SimpleResponse{
			Status:  http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to get forecast: %s", err.Error()),
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
		return
	}
	err = json.NewEncoder(w).Encode(events)
	if err != nil {
		return
	}
}

// handleGetLeaderStatus returns leadership status
func (s *Server) handleGetLeaderStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]bool{
		"isLeader": s.forecastService.IsLeader(),
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}
