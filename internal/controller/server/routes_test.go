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

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

const testOrigins = "https://console.example.com"

var (
	testNode = types.Node{
		ID:   "n-1",
		Name: "node-1",
		Status: types.Status{
			Condition: types.Condition{Status: types.Ready, Healthy: true},
			Capacity:  types.Resources{CPU: utils.FromInt32(8), Memory: utils.FromInt32(16384), Runners: utils.FromInt32(2)},
		},
	}
	testRunner = types.Runner{
		ID:        "r-1",
		NodeID:    "n-1",
		Condition: types.RunnerCondition{Status: types.Running},
		Config:    types.RunnerConfig{Name: "runner-1", BaseImage: "macos:latest"},
	}
	testRegistration = types.NodeRegistrationRequest{
		NodeName: "node-1",
		Capacity: types.Resources{CPU: utils.FromInt32(8), Memory: utils.FromInt32(16384), Runners: utils.FromInt32(2)},
	}
	testHeartbeat = types.NodeHeartbeatRequest{
		NodeName:    "node-1",
		Status:      types.Ready,
		Allocatable: types.Resources{CPU: utils.FromInt32(4), Memory: utils.FromInt32(8192), Runners: utils.FromInt32(1)},
	}
	testRunnerUpdate = types.RunnerStatusUpdate{ID: "r-1", Status: types.Running, GhaRunnerName: "gha-1"}
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func serve(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)))
	return w
}

func TestControllerRoutesSuccess(t *testing.T) {
	cases := []struct {
		setup      func(f *FakeController)
		name       string
		method     string
		target     string
		body       string
		wantBody   string
		wantStatus int
	}{
		{
			name:       "health",
			method:     http.MethodGet,
			target:     "/api/v1/health",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":200,"message":"Healthy"}`,
		},
		{
			name:       "list nodes",
			method:     http.MethodGet,
			target:     "/api/v1/nodes",
			setup:      func(f *FakeController) { f.On("GetNodeList").Return([]types.Node{testNode}, nil) },
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, []types.Node{testNode}),
		},
		{
			name:   "register node",
			method: http.MethodPost,
			target: "/api/v1/nodes/register",
			body:   mustJSON(t, testRegistration),
			setup: func(f *FakeController) {
				f.On("RegisterNewNode", testRegistration).Return(types.Registered, &testNode, nil)
			},
			wantStatus: http.StatusCreated,
			wantBody: mustJSON(t, types.NodeRegistrationResponse{
				RegistrationStatus: types.Registered,
				Node:               testNode,
				Message:            "Node node-1 registered with id n-1",
			}),
		},
		{
			name:       "get node",
			method:     http.MethodGet,
			target:     "/api/v1/nodes/n-1",
			setup:      func(f *FakeController) { f.On("GetNodeInfo", utils.UID("n-1")).Return(&testNode, nil) },
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, testNode),
		},
		{
			name:       "get node named register",
			method:     http.MethodGet,
			target:     "/api/v1/nodes/register",
			setup:      func(f *FakeController) { f.On("GetNodeInfo", utils.UID("register")).Return(&testNode, nil) },
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, testNode),
		},
		{
			name:       "get node with encoded space in id",
			method:     http.MethodGet,
			target:     "/api/v1/nodes/n%201",
			setup:      func(f *FakeController) { f.On("GetNodeInfo", utils.UID("n 1")).Return(&testNode, nil) },
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, testNode),
		},
		{
			name:   "node heartbeat",
			method: http.MethodPut,
			target: "/api/v1/nodes/n-1/status",
			body:   mustJSON(t, testHeartbeat),
			setup: func(f *FakeController) {
				f.On("ProcessNodeHeartBeat", utils.UID("n-1"), testHeartbeat).Return(types.HeartbeatSuccess, &testNode, nil)
			},
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, testNode),
		},
		{
			name:       "deregister node",
			method:     http.MethodPut,
			target:     "/api/v1/nodes/n-1/deregister",
			setup:      func(f *FakeController) { f.On("DeregisterNode", utils.UID("n-1")).Return(&testNode, nil) },
			wantStatus: http.StatusOK,
			wantBody:   `{"status":200,"message":"Node n-1 disabled"}`,
		},
		{
			name:       "enable node",
			method:     http.MethodPut,
			target:     "/api/v1/nodes/n-1/enable",
			setup:      func(f *FakeController) { f.On("ReEnable", utils.UID("n-1")).Return(&testNode, nil) },
			wantStatus: http.StatusOK,
			wantBody:   `{"status":200,"message":"Node n-1 disabled"}`,
		},
		{
			name:       "disable node",
			method:     http.MethodPut,
			target:     "/api/v1/nodes/n-1/disable",
			setup:      func(f *FakeController) { f.On("DisableNode", utils.UID("n-1")).Return(&testNode, nil) },
			wantStatus: http.StatusOK,
			wantBody:   `{"status":200,"message":"Node n-1 disabled"}`,
		},
		{
			name:       "list runners",
			method:     http.MethodGet,
			target:     "/api/v1/runners",
			setup:      func(f *FakeController) { f.On("GetRunnerList").Return([]types.Runner{testRunner}, nil) },
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, []types.Runner{testRunner}),
		},
		{
			name:       "get runner",
			method:     http.MethodGet,
			target:     "/api/v1/runners/r-1",
			setup:      func(f *FakeController) { f.On("GetRunnerInfo", utils.UID("r-1")).Return(&testRunner, nil) },
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, testRunner),
		},
		{
			name:       "update runner status",
			method:     http.MethodPut,
			target:     "/api/v1/runners/r-1/status",
			body:       mustJSON(t, testRunnerUpdate),
			setup:      func(f *FakeController) { f.On("ProcessRunnersStatusUpdate", testRunnerUpdate).Return(&testRunner, nil) },
			wantStatus: http.StatusOK,
			wantBody:   mustJSON(t, testRunner),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &FakeController{}
			if tc.setup != nil {
				tc.setup(f)
			}

			w := serve(NewServer(f, testOrigins), tc.method, tc.target, tc.body)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.JSONEq(t, tc.wantBody, w.Body.String())
			f.AssertExpectations(t)
		})
	}
}

func TestControllerRoutesNotFound(t *testing.T) {
	cases := []struct {
		method string
		target string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/api/v1"},
		{http.MethodGet, "/api/v1/"},
		{http.MethodGet, "/api/v1/unknown"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/nodes"},
		{http.MethodGet, "/runners"},
		{http.MethodPost, "/nodes/register"},
		{http.MethodGet, "/nodes/n-1"},
		{http.MethodGet, "/api/v1/ws/watch"},
		{http.MethodGet, "/api/v1/health/"},
		{http.MethodGet, "/api/v1/nodes/"},
		{http.MethodGet, "/api/v1/runners/"},
		{http.MethodGet, "/api/v1/nodes/n-1/"},
		{http.MethodGet, "/api/v1/runners/r-1/"},
		{http.MethodPost, "/api/v1/nodes/register/"},
		{http.MethodPut, "/api/v1/nodes/n-1/status/"},
		{http.MethodPut, "/api/v1/runners/r-1/status/"},
		{http.MethodGet, "/ws/watch/"},
		{http.MethodGet, "/api/v1/nodes/n-1/unknown"},
		{http.MethodGet, "/api/v1/nodes/a%2Fb"},
		{http.MethodPut, "/api/v1/nodes/a%2Fb/status"},
		{http.MethodGet, "/api/v1/runners/a%2Fb"},
		{http.MethodPut, "/api/v1/runners/a%2Fb/status"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			f := &FakeController{}

			w := serve(NewServer(f, testOrigins), tc.method, tc.target, "")

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Empty(t, w.Header().Get("Location"))
			f.AssertExpectations(t)
		})
	}
}

func TestControllerRoutesCleanPathRedirect(t *testing.T) {
	cases := []struct {
		method       string
		target       string
		wantLocation string
	}{
		{http.MethodGet, "//api/v1/nodes", "/api/v1/nodes"},
		{http.MethodGet, "/api/v1//nodes", "/api/v1/nodes"},
		{http.MethodGet, "/api/v1/nodes//n-1", "/api/v1/nodes/n-1"},
		{http.MethodPut, "/api/v1/nodes//status", "/api/v1/nodes/status"},
		{http.MethodPost, "//api/v1/nodes/register", "/api/v1/nodes/register"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			f := &FakeController{}

			w := serve(NewServer(f, testOrigins), tc.method, tc.target, "")

			assert.Equal(t, http.StatusMovedPermanently, w.Code)
			assert.Equal(t, tc.wantLocation, w.Header().Get("Location"))
			f.AssertExpectations(t)
		})
	}
}

func TestControllerWatchRouteWrongMethod(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			f := &FakeController{}

			w := serve(NewServer(f, testOrigins), method, "/ws/watch", "")

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			f.AssertExpectations(t)
		})
	}
}

func TestControllerRoutesMalformedJSON(t *testing.T) {
	cases := []struct {
		target      string
		wantMessage string
		method      string
	}{
		{"/api/v1/nodes/register", "Failed to read registration request", http.MethodPost},
		{"/api/v1/nodes/n-1/status", "Failed to read node heartbeat request", http.MethodPut},
		{"/api/v1/runners/r-1/status", "Failed to read runner status update request", http.MethodPut},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			f := &FakeController{}

			w := serve(NewServer(f, testOrigins), tc.method, tc.target, "{not json")

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantMessage, body["message"])
			f.AssertExpectations(t)
		})
	}
}

func TestControllerCORSHeaders(t *testing.T) {
	cases := []struct {
		setup  func(f *FakeController)
		method string
		target string
	}{
		{func(f *FakeController) { f.On("GetNodeList").Return([]types.Node{}, nil) }, http.MethodGet, "/api/v1/nodes"},
		{nil, http.MethodOptions, "/api/v1/nodes"},
		{func(f *FakeController) { f.On("GetRunnerList").Return([]types.Runner{}, nil) }, http.MethodGet, "/api/v1/runners"},
		{nil, http.MethodOptions, "/api/v1/runners"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			f := &FakeController{}
			if tc.setup != nil {
				tc.setup(f)
			}

			w := serve(NewServer(f, testOrigins), tc.method, tc.target, "")

			assert.Equal(t, http.StatusOK, w.Code)
			assert.NotContains(t, w.Header(), "Access-Control-Allow-Methods")
			assert.Equal(t, testOrigins, w.Header().Get("Access-Control-Allow-Origin"))
			f.AssertExpectations(t)
		})
	}
}

func TestControllerPreflightHasEmptyBody(t *testing.T) {
	for _, target := range []string{"/api/v1/nodes", "/api/v1/runners"} {
		t.Run(target, func(t *testing.T) {
			f := &FakeController{}

			w := serve(NewServer(f, testOrigins), http.MethodOptions, target, "")

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Empty(t, w.Body.String())
			f.AssertExpectations(t)
		})
	}
}

func TestControllerNoCORSHeadersOutsideListRoutes(t *testing.T) {
	f := &FakeController{}
	f.On("GetNodeInfo", utils.UID("n-1")).Return(&testNode, nil)
	f.On("GetRunnerInfo", utils.UID("r-1")).Return(&testRunner, nil)
	h := NewServer(f, testOrigins)

	for _, target := range []string{"/api/v1/health", "/api/v1/nodes/n-1", "/api/v1/runners/r-1"} {
		t.Run(target, func(t *testing.T) {
			w := serve(h, http.MethodGet, target, "")

			assert.Equal(t, http.StatusOK, w.Code)
			assert.NotContains(t, w.Header(), "Access-Control-Allow-Methods")
			assert.NotContains(t, w.Header(), "Access-Control-Allow-Origin")
		})
	}
	f.AssertExpectations(t)
}
