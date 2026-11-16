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
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func observeRequestLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	obsCore, logs := observer.New(zapcore.DebugLevel)
	t.Cleanup(logger.SetLogger(zap.New(obsCore)))
	return logs
}

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

func TestHTTPServerTurnsPanicInto500(t *testing.T) {
	logs := observeRequestLogs(t)
	f := &FakeController{}
	f.On("GetNodeList").Run(func(mock.Arguments) { panic("boom") })
	s := NewHTTPServer(f, net.ParseIP("127.0.0.1"), "8043", "*")

	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/nodes", http.NoBody))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assertDebugRequestLog(t, logs, http.MethodGet, "/api/v1/nodes", http.StatusInternalServerError)
}

func TestHTTPServerLogsRequestAtDebug(t *testing.T) {
	logs := observeRequestLogs(t)
	s := NewHTTPServer(&FakeController{}, net.ParseIP("127.0.0.1"), "8043", "*")

	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", http.NoBody))

	assert.Equal(t, http.StatusOK, w.Code)
	assertDebugRequestLog(t, logs, http.MethodGet, "/api/v1/health", http.StatusOK)
}

func TestHTTPServerWatchUpgradesThroughMiddlewarePastWriteTimeout(t *testing.T) {
	f := newWatchFake()
	srv := httptest.NewUnstartedServer(NewHTTPServer(f, net.ParseIP("127.0.0.1"), "8043", "*").Handler)
	srv.Config.WriteTimeout = time.Second
	srv.Start()
	defer srv.Close()

	conn, status, err := dialWatch(t, srv, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusSwitchingProtocols, status)

	time.Sleep(2 * time.Second)

	receiveUpdates(t, f, conn)
}
