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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

func TestHeadOnGetRoutes(t *testing.T) {
	f := &fakeTarter{}
	f.On("ListActiveRunners").Return(testRunnerList)
	f.On("GetRunnerState", utils.UID("r-1")).Return(testRunnerState, true)
	f.On("ListRunnerWorkers").Return([]string{"w-1"})
	f.On("GetWorkersCount").Return(1)
	f.On("GetStandaloneConfig").Return(testStandaloneConfig)
	srv := httptest.NewServer(newTestRouter(f))
	defer srv.Close()

	for _, target := range []string{"/runners", "/runners/r-1", "/sys/workers", "/sys/workers-count", "/sys/standalone/config"} {
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
