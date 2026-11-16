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
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

func TestHeadOnGetRoutes(t *testing.T) {
	f := &FakeController{}
	f.On("GetNodeList").Return([]types.Node{testNode}, nil)
	f.On("GetNodeInfo", utils.UID("n-1")).Return(&testNode, nil)
	f.On("GetRunnerList").Return([]types.Runner{testRunner}, nil)
	f.On("GetRunnerInfo", utils.UID("r-1")).Return(&testRunner, nil)
	srv := httptest.NewServer(NewServer(f, testOrigins))
	defer srv.Close()

	for _, target := range []string{"/api/v1/health", "/api/v1/nodes", "/api/v1/nodes/n-1", "/api/v1/runners", "/api/v1/runners/r-1"} {
		t.Run(target, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, srv.URL+target, http.NoBody)
			require.NoError(t, err)

			resp, err := srv.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Empty(t, body)
		})
	}
	f.AssertExpectations(t)
}

func TestControllerAPIRoutesWrongMethod(t *testing.T) {
	routes := []struct {
		path    string
		methods []string
	}{
		{"/api/v1/health", []string{http.MethodGet}},
		{"/api/v1/nodes", []string{http.MethodGet, http.MethodOptions}},
		{"/api/v1/nodes/register", []string{http.MethodPost, http.MethodGet}},
		{"/api/v1/nodes/n-1", []string{http.MethodGet}},
		{"/api/v1/nodes/n-1/status", []string{http.MethodPut}},
		{"/api/v1/nodes/n-1/deregister", []string{http.MethodPut}},
		{"/api/v1/nodes/n-1/enable", []string{http.MethodPut}},
		{"/api/v1/nodes/n-1/disable", []string{http.MethodPut}},
		{"/api/v1/runners", []string{http.MethodGet, http.MethodOptions}},
		{"/api/v1/runners/r-1", []string{http.MethodGet}},
		{"/api/v1/runners/r-1/status", []string{http.MethodPut}},
	}
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions}
	for _, route := range routes {
		for _, method := range methods {
			if slices.Contains(route.methods, method) {
				continue
			}
			t.Run(method+" "+route.path, func(t *testing.T) {
				f := &FakeController{}

				w := serve(NewServer(f, testOrigins), method, route.path, "")

				assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
				f.AssertExpectations(t)
			})
		}
	}
}

func TestHeadOnWatchNotAllowed(t *testing.T) {
	f := &FakeController{}
	srv := httptest.NewServer(NewServer(f, testOrigins))
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, srv.URL+"/ws/watch", http.NoBody)
	require.NoError(t, err)

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	assert.Equal(t, http.MethodGet, resp.Header.Get("Allow"))
	f.AssertExpectations(t)
}
