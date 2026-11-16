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
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/config"
	tt "github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type fakeTarter struct {
	mock.Mock
}

func (f *fakeTarter) ListRunners() []tt.RunnerStateList {
	return f.Called().Get(0).([]tt.RunnerStateList)
}

func (f *fakeTarter) ListActiveRunners() []tt.RunnerStateList {
	return f.Called().Get(0).([]tt.RunnerStateList)
}

func (f *fakeTarter) GetRunnerState(id utils.UID) (tt.RunnerState, bool) {
	args := f.Called(id)
	return args.Get(0).(tt.RunnerState), args.Bool(1)
}

func (f *fakeTarter) StopRunner(id utils.UID) bool {
	return f.Called(id).Bool(0)
}

func (f *fakeTarter) GetWorkersCount() int {
	return f.Called().Int(0)
}

func (f *fakeTarter) ListRunnerWorkers() []string {
	return f.Called().Get(0).([]string)
}

func (f *fakeTarter) GetStandaloneConfig() config.Config {
	return f.Called().Get(0).(config.Config)
}

func (f *fakeTarter) RestartStandaloneRunners() {
	f.Called()
}

var (
	testRunnerList  = []tt.RunnerStateList{{ID: "r-1", Name: "runner-1", TartVMName: "vm-1", Status: "running"}}
	testAllRunners  = []tt.RunnerStateList{testRunnerList[0], {ID: "r-0", Name: "runner-0", Status: "finished"}}
	testRunnerState = tt.RunnerState{
		Runner: &tt.Runner{ID: "r-1", TartVMName: "vm-1", Config: types.RunnerConfig{Name: "runner-1"}},
		Status: tt.Running,
	}
	testStandaloneConfig = &config.StandaloneConfig{
		Runners:      []types.RunnerConfig{{Name: "runner-1", BaseImage: "macos:latest"}},
		NodeCapacity: config.NodeCapacity{CPU: utils.FromInt32(8)},
	}
	tarterRoutes = []struct {
		path    string
		methods []string
	}{
		{"/runners", []string{http.MethodGet}},
		{"/runners/r-1", []string{http.MethodGet}},
		{"/runners/r-1/stop", []string{http.MethodPut}},
		{"/sys/workers", []string{http.MethodGet}},
		{"/sys/workers-count", []string{http.MethodGet}},
		{"/sys/standalone/config", []string{http.MethodGet}},
		{"/sys/standalone/restart", []string{http.MethodPut}},
	}
	sweepMethods = []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions,
	}
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func newTestRouter(f *fakeTarter) http.Handler {
	r := NewRouter()
	SetupRoutes(r, f)
	SetupSysRoutes(r, f)
	SetupStandaloneTarterRoutes(r, f)
	return r
}

func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, http.NoBody))
	return w
}

func TestTarterRoutesSuccess(t *testing.T) {
	cases := []struct {
		setup    func(f *fakeTarter)
		name     string
		method   string
		target   string
		wantBody string
	}{
		{
			name:     "list active runners",
			method:   http.MethodGet,
			target:   "/runners",
			setup:    func(f *fakeTarter) { f.On("ListActiveRunners").Return(testRunnerList) },
			wantBody: mustJSON(t, testRunnerList),
		},
		{
			name:     "list all runners",
			method:   http.MethodGet,
			target:   "/runners?status=all",
			setup:    func(f *fakeTarter) { f.On("ListRunners").Return(testAllRunners) },
			wantBody: mustJSON(t, testAllRunners),
		},
		{
			name:     "get runner",
			method:   http.MethodGet,
			target:   "/runners/r-1",
			setup:    func(f *fakeTarter) { f.On("GetRunnerState", utils.UID("r-1")).Return(testRunnerState, true) },
			wantBody: mustJSON(t, testRunnerState),
		},
		{
			name:     "get runner with encoded space in id",
			method:   http.MethodGet,
			target:   "/runners/r%201",
			setup:    func(f *fakeTarter) { f.On("GetRunnerState", utils.UID("r 1")).Return(testRunnerState, true) },
			wantBody: mustJSON(t, testRunnerState),
		},
		{
			name:     "stop runner",
			method:   http.MethodPut,
			target:   "/runners/r-1/stop",
			setup:    func(f *fakeTarter) { f.On("StopRunner", utils.UID("r-1")).Return(true) },
			wantBody: `{"message":"Runner with id r-1 is stopping"}`,
		},
		{
			name:     "list workers",
			method:   http.MethodGet,
			target:   "/sys/workers",
			setup:    func(f *fakeTarter) { f.On("ListRunnerWorkers").Return([]string{"w-1", "w-2"}) },
			wantBody: `["w-1","w-2"]`,
		},
		{
			name:     "workers count",
			method:   http.MethodGet,
			target:   "/sys/workers-count",
			setup:    func(f *fakeTarter) { f.On("GetWorkersCount").Return(3) },
			wantBody: `{"count": 3}`,
		},
		{
			name:     "standalone config",
			method:   http.MethodGet,
			target:   "/sys/standalone/config",
			setup:    func(f *fakeTarter) { f.On("GetStandaloneConfig").Return(testStandaloneConfig) },
			wantBody: mustJSON(t, testStandaloneConfig),
		},
		{
			name:     "restart standalone runners",
			method:   http.MethodPut,
			target:   "/sys/standalone/restart",
			setup:    func(f *fakeTarter) { f.On("RestartStandaloneRunners").Return() },
			wantBody: `{"message":"Standalone config restarted"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeTarter{}
			tc.setup(f)

			w := serve(newTestRouter(f), tc.method, tc.target)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.JSONEq(t, tc.wantBody, w.Body.String())
			f.AssertExpectations(t)
		})
	}
}

func TestTarterRoutesWrongMethod(t *testing.T) {
	for _, route := range tarterRoutes {
		for _, method := range sweepMethods {
			if slices.Contains(route.methods, method) {
				continue
			}
			t.Run(method+" "+route.path, func(t *testing.T) {
				f := &fakeTarter{}

				w := serve(newTestRouter(f), method, route.path)

				assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
				f.AssertExpectations(t)
			})
		}
	}
}

func TestTarterRoutesNotFound(t *testing.T) {
	cases := []struct {
		method string
		target string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/unknown"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/api/v1/runners"},
		{http.MethodGet, "/sys"},
		{http.MethodGet, "/sys/standalone"},
		{http.MethodGet, "/runners/"},
		{http.MethodGet, "/runners/r-1/"},
		{http.MethodPut, "/runners/r-1/stop/"},
		{http.MethodGet, "/sys/workers/"},
		{http.MethodGet, "/sys/workers-count/"},
		{http.MethodGet, "/sys/standalone/config/"},
		{http.MethodPut, "/sys/standalone/restart/"},
		{http.MethodGet, "/runners/r-1/unknown"},
		{http.MethodGet, "/runners/a%2Fb"},
		{http.MethodPut, "/runners/a%2Fb/stop"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			f := &fakeTarter{}

			w := serve(newTestRouter(f), tc.method, tc.target)

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Empty(t, w.Header().Get("Location"))
			f.AssertExpectations(t)
		})
	}
}

func TestTarterRoutesCleanPathRedirect(t *testing.T) {
	cases := []struct {
		method       string
		target       string
		wantLocation string
	}{
		{http.MethodGet, "//runners", "/runners"},
		{http.MethodGet, "/sys//workers", "/sys/workers"},
		{http.MethodPut, "/runners//stop", "/runners/stop"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			f := &fakeTarter{}

			w := serve(newTestRouter(f), tc.method, tc.target)

			assert.Equal(t, http.StatusMovedPermanently, w.Code)
			assert.Equal(t, tc.wantLocation, w.Header().Get("Location"))
			f.AssertExpectations(t)
		})
	}
}
