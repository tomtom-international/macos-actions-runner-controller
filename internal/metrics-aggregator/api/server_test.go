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
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
)

// MockMetricsAggregatorService is a mock implementation of the MetricsAggregatorService interface
type MockMetricsAggregatorService struct {
	mock.Mock
}

func (m *MockMetricsAggregatorService) CollectRunnersMetrics() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func TestHandleGetMetricsSuccess(t *testing.T) {
	mockService := new(MockMetricsAggregatorService)
	expectedMetrics := "metric1{label=\"value\"} 42\nmetric2{label=\"value\"} 84"
	mockService.On("CollectRunnersMetrics").Return(expectedMetrics, nil)

	server := &Server{maService: mockService}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	server.handleGetMetrics(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; version=0.0.4; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Equal(t, expectedMetrics, string(body))
	mockService.AssertExpectations(t)
}

func TestHandleGetMetricsError(t *testing.T) {
	mockService := new(MockMetricsAggregatorService)
	expectedError := errors.New("failed to collect metrics")
	mockService.On("CollectRunnersMetrics").Return("", expectedError)

	server := &Server{maService: mockService}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	server.handleGetMetrics(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var errorResp core.SimpleResponse
	err := json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, errorResp.Status)
	assert.Contains(t, errorResp.Message, expectedError.Error())
	mockService.AssertExpectations(t)
}

func TestNewServer(t *testing.T) {
	mockService := new(MockMetricsAggregatorService)
	address := net.ParseIP("127.0.0.1")
	port := "8080"

	server := NewServer(mockService, address, port)

	assert.NotNil(t, server)
	assert.Equal(t, mockService, server.maService)
	assert.Equal(t, "127.0.0.1:8080", server.server.Addr)
}

func TestSetupRoutes(t *testing.T) {
	mockService := new(MockMetricsAggregatorService)
	server := &Server{maService: mockService}
	mux := http.NewServeMux()

	server.setupRoutes(mux)

	metricsHandler, _ := mux.Handler(httptest.NewRequest(http.MethodGet, "/metrics", nil))
	healthHandler, _ := mux.Handler(httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.NotNil(t, metricsHandler)
	assert.NotNil(t, healthHandler)
}
