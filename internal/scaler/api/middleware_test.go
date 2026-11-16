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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger/loggertest"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func assertDebugRequestLog(t *testing.T, logs *observer.ObservedLogs, method, path string, status int) {
	t.Helper()
	prefix := fmt.Sprintf("%s %s %d ", method, path, status)
	entries := logs.Filter(func(e observer.LoggedEntry) bool { return strings.HasPrefix(e.Message, prefix) }).All()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.DebugLevel, entries[0].Level)
	_, err := time.ParseDuration(strings.TrimPrefix(entries[0].Message, prefix))
	assert.NoError(t, err)
	assert.Zero(t, logs.FilterLevelExact(zapcore.InfoLevel).Len())
}

func TestServerTurnsPanicInto500(t *testing.T) {
	logs := loggertest.Observe(t, zapcore.DebugLevel)
	svc := new(MockForecastService)
	svc.On("IsLeader").Run(func(mock.Arguments) { panic("boom") })
	server := NewServer(svc, net.ParseIP("127.0.0.1"), "8045")

	w := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/leader", http.NoBody))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assertDebugRequestLog(t, logs, http.MethodGet, "/api/leader", http.StatusInternalServerError)
}

func TestServerLogsRequestAtDebug(t *testing.T) {
	logs := loggertest.Observe(t, zapcore.DebugLevel)
	server := NewServer(new(MockForecastService), net.ParseIP("127.0.0.1"), "8045")

	w := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	assert.Equal(t, http.StatusOK, w.Code)
	assertDebugRequestLog(t, logs, http.MethodGet, "/health", http.StatusOK)
}
