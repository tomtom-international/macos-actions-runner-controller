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
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/forecast"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
)

// MockForecastService is a mock implementation of ForecastService
type MockForecastService struct {
	mock.Mock
}

func (m *MockForecastService) GetNextScalerEvent() ([]forecast.Event, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]forecast.Event), args.Error(1)
}

func (m *MockForecastService) GetUpcomingScalerEvents(days int, tz *time.Location) ([]forecast.Event, error) {
	args := m.Called(days, tz)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]forecast.Event), args.Error(1)
}

func (m *MockForecastService) IsLeader() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestGetForecast(t *testing.T) {
	mockService := new(MockForecastService)

	events := []forecast.Event{
		{
			ScalingGroup:     "test-group",
			ScheduleName:     "scale-down",
			TriggerTime:      time.Now().Add(24 * time.Hour),
			TriggerTimeHuman: time.Now().Add(24 * time.Hour).Format("Mon, 02 Jan 2006 3:04 PM (MST)"),
			Action: struct {
				DesiredCapacity int `json:"desired_capacity"`
			}{
				DesiredCapacity: 2,
			},
		},
	}

	mockService.On("GetNextScalerEvent").Return(events, nil)
	mockService.On("GetUpcomingScalerEvents", 7, time.UTC).Return(events, nil)
	mockService.On("GetUpcomingScalerEvents", 14, time.UTC).Return(events, nil)

	server := NewServer(mockService, net.ParseIP("127.0.0.1"), "8040")

	t.Run("Default parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/forecast", nil)
		w := httptest.NewRecorder()

		server.handleGetForecast(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var result []forecast.Event
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		require.Len(t, result, len(events))
		for i, evt := range events {
			assert.Equal(t, evt.ScalingGroup, result[i].ScalingGroup)
			assert.Equal(t, evt.ScheduleName, result[i].ScheduleName)
			assert.Equal(t, evt.Action.DesiredCapacity, result[i].Action.DesiredCapacity)
			assert.WithinDuration(t, evt.TriggerTime, result[i].TriggerTime, time.Millisecond)
		}
	})

	// Test with days parameter
	t.Run("With days parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/forecast?days=7", nil)
		w := httptest.NewRecorder()

		server.handleGetForecast(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var result []forecast.Event
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		require.Len(t, result, len(events))
		for i, evt := range events {
			assert.Equal(t, evt.ScalingGroup, result[i].ScalingGroup)
			assert.Equal(t, evt.ScheduleName, result[i].ScheduleName)
			assert.Equal(t, evt.Action.DesiredCapacity, result[i].Action.DesiredCapacity)
			// Compare time.Time values with a small epsilon for tolerance
			assert.WithinDuration(t, evt.TriggerTime, result[i].TriggerTime, time.Millisecond)
		}
	})

	// Test with invalid days parameter
	t.Run("Invalid days parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/forecast?days=invalid", nil)
		w := httptest.NewRecorder()

		server.handleGetForecast(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var response core.SimpleResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, response.Status)
		assert.Contains(t, response.Message, "Invalid 'days' parameter")
	})

	// Test without of range days parameter
	t.Run("Out of range days parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/forecast?days=50", nil)
		w := httptest.NewRecorder()

		server.handleGetForecast(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var response core.SimpleResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, response.Status)
		assert.Contains(t, response.Message, "Invalid 'days' parameter")
	})

	// Test with timezone parameter
	t.Run("With timezone parameter", func(t *testing.T) {
		// Set up expectation for different timezone
		est, _ := time.LoadLocation("America/New_York")
		mockService.On("GetUpcomingScalerEvents", 7, est).Return(events, nil)

		req := httptest.NewRequest("GET", "/api/forecast?days=7&tz=America/New_York", nil)
		w := httptest.NewRecorder()

		server.handleGetForecast(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var result []forecast.Event
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		for i, evt := range events {
			assert.Equal(t, evt.ScalingGroup, result[i].ScalingGroup)
			assert.Equal(t, evt.ScheduleName, result[i].ScheduleName)
			assert.Equal(t, evt.Action.DesiredCapacity, result[i].Action.DesiredCapacity)
			// Compare time.Time values with a small epsilon for tolerance
			assert.WithinDuration(t, evt.TriggerTime, result[i].TriggerTime, time.Millisecond)
		}
	})

	// Test with invalid timezone parameter
	t.Run("Invalid timezone parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/forecast?days=7&tz=InvalidTimezone", nil)
		w := httptest.NewRecorder()

		server.handleGetForecast(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var response core.SimpleResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, response.Status)
		assert.Contains(t, response.Message, "Invalid 'tz' parameter")
	})
}

func TestHealthEndpoint(t *testing.T) {
	handler := http.HandlerFunc(core.SimpleHealthcheckHandler)

	// Test health endpoint
	t.Run("Health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response core.SimpleResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, response.Status)
		assert.Equal(t, "Healthy", response.Message)
	})
}

func TestLeaderEndpoint(t *testing.T) {
	mockService := new(MockForecastService)

	server := NewServer(mockService, net.ParseIP("127.0.0.1"), "8040")

	// Test when service is leader
	t.Run("IsLeader returns true", func(t *testing.T) {
		mockService.On("IsLeader").Return(true).Once()

		req := httptest.NewRequest("GET", "/api/leader", nil)
		w := httptest.NewRecorder()

		server.handleGetLeaderStatus(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]bool
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["isLeader"])
	})

	// Test when service is not leader
	t.Run("IsLeader returns false", func(t *testing.T) {
		mockService.On("IsLeader").Return(false).Once()

		req := httptest.NewRequest("GET", "/api/leader", nil)
		w := httptest.NewRecorder()

		server.handleGetLeaderStatus(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]bool
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.False(t, response["isLeader"])
	})

	mockService.AssertExpectations(t)
}

func TestServerLifecycle(t *testing.T) {
	mockService := new(MockForecastService)

	server := NewServer(mockService, net.ParseIP("127.0.0.1"), "0")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		server.server.Addr = listener.Addr().String()

		err := server.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestSetupRoutes(t *testing.T) {
	mockService := new(MockForecastService)
	mockService.On("GetNextScalerEvent").Return([]forecast.Event{}, nil)
	mockService.On("IsLeader").Return(true)

	server := NewServer(mockService, net.ParseIP("127.0.0.1"), "8040")

	testServer := httptest.NewServer(server.server.Handler)
	defer testServer.Close()

	// Test that routes are properly set up
	t.Run("Routes are set up", func(t *testing.T) {
		// Test the forecast endpoint
		resp, err := http.Get(fmt.Sprintf("%s/api/forecast", testServer.URL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Test the leader endpoint
		resp, err = http.Get(fmt.Sprintf("%s/api/leader", testServer.URL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Test the health endpoint
		resp, err = http.Get(fmt.Sprintf("%s/health", testServer.URL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})
}
