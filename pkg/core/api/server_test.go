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
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func observeLogs(t *testing.T, level zapcore.Level) *observer.ObservedLogs {
	t.Helper()
	obsCore, logs := observer.New(level)
	t.Cleanup(logger.SetLogger(zap.New(obsCore)))
	return logs
}

func requestLogs(logs *observer.ObservedLogs, method, path string) []observer.LoggedEntry {
	prefix := method + " " + path + " "
	return logs.Filter(func(e observer.LoggedEntry) bool { return strings.HasPrefix(e.Message, prefix) }).All()
}

func assertRequestLog(t *testing.T, logs *observer.ObservedLogs, method, path string, status int) {
	t.Helper()
	entries := requestLogs(logs, method, path)
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.DebugLevel, entries[0].Level)
	fields := strings.Fields(entries[0].Message)
	require.Len(t, fields, 4)
	assert.Equal(t, strconv.Itoa(status), fields[2])
	_, err := time.ParseDuration(fields[3])
	assert.NoError(t, err)
}

func TestNewHTTPServerFields(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	s := NewHTTPServer("127.0.0.1:9000", h, 3*time.Second, 5*time.Second, 7*time.Second)

	assert.Equal(t, "127.0.0.1:9000", s.Addr)
	assert.Equal(t, 3*time.Second, s.ReadTimeout)
	assert.Equal(t, 5*time.Second, s.WriteTimeout)
	assert.Equal(t, 7*time.Second, s.IdleTimeout)
	assert.Equal(t, 1<<20, s.MaxHeaderBytes)
	assert.Zero(t, s.ReadHeaderTimeout)
	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	assert.Equal(t, http.StatusTeapot, w.Code)
}

func TestMiddlewareLogsRequestAtDebug(t *testing.T) {
	logs := observeLogs(t, zapcore.DebugLevel)
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/items/42", http.NoBody))

	assert.Equal(t, http.StatusCreated, w.Code)
	assertRequestLog(t, logs, http.MethodPost, "/items/42", http.StatusCreated)
}

func TestMiddlewareLogsNothingAtInfo(t *testing.T) {
	logs := observeLogs(t, zapcore.InfoLevel)
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items", http.NoBody))

	assert.Zero(t, logs.Len())
}

func TestMiddlewareLogsImplicitOK(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"/empty": func(http.ResponseWriter, *http.Request) {},
		"/body":  func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) },
	}
	for path, handler := range cases {
		t.Run(path, func(t *testing.T) {
			logs := observeLogs(t, zapcore.DebugLevel)

			w := httptest.NewRecorder()
			Middleware(handler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, http.NoBody))

			assert.Equal(t, http.StatusOK, w.Code)
			assertRequestLog(t, logs, http.MethodGet, path, http.StatusOK)
		})
	}
}

func TestMiddlewareTurnsPanicInto500(t *testing.T) {
	logs := observeLogs(t, zapcore.DebugLevel)
	h := Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))

	w := httptest.NewRecorder()
	require.NotPanics(t, func() { h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", http.NoBody)) })

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assertRequestLog(t, logs, http.MethodGet, "/panic", http.StatusInternalServerError)
	assert.Equal(t, 1, logs.FilterLevelExact(zapcore.ErrorLevel).FilterMessageSnippet("boom").Len())
}

func TestMiddlewareRepanicsErrAbortHandler(t *testing.T) {
	logs := observeLogs(t, zapcore.DebugLevel)
	h := Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/abort", http.NoBody))
	})
	assert.Len(t, requestLogs(logs, http.MethodGet, "/abort"), 1)
}

func TestMiddlewareAbortsOnPanicAfterHeadersWritten(t *testing.T) {
	observeLogs(t, zapcore.DebugLevel)
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("late")
	}))

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/late", http.NoBody))
	})
}

func TestMiddlewareWriterUnwrapsForResponseController(t *testing.T) {
	observeLogs(t, zapcore.DebugLevel)
	var flushErr error
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flushErr = http.NewResponseController(w).Flush()
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/flush", http.NoBody))

	assert.NoError(t, flushErr)
	assert.True(t, w.Flushed)
}

func TestMiddlewareKeepsWebsocketUpgrade(t *testing.T) {
	logs := observeLogs(t, zapcore.DebugLevel)
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.WriteMessage(msgType, msg)
	})
	srv := httptest.NewServer(Middleware(echo))
	defer srv.Close()

	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()
	resp.Body.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("ping")))
	_, got, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "ping", string(got))

	require.Eventually(t, func() bool { return len(requestLogs(logs, http.MethodGet, "/ws")) == 1 }, 5*time.Second, 10*time.Millisecond)
	assertRequestLog(t, logs, http.MethodGet, "/ws", http.StatusSwitchingProtocols)
}
