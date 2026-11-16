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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPServerConfig(t *testing.T) {
	s := NewHTTPServer(nil, net.ParseIP("127.0.0.1"), "8043", "*")

	assert.Equal(t, "127.0.0.1:8043", s.Addr)
	assert.Equal(t, 120*time.Second, s.ReadTimeout)
	assert.Equal(t, 60*time.Second, s.WriteTimeout)
	assert.Equal(t, 120*time.Second, s.IdleTimeout)
	assert.Equal(t, 1<<20, s.MaxHeaderBytes)
	assert.Zero(t, s.ReadHeaderTimeout)
	require.NotNil(t, s.Handler)
}

func TestNewHTTPServerServesControllerRoutes(t *testing.T) {
	s := NewHTTPServer(nil, net.ParseIP("127.0.0.1"), "8043", "*")

	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", http.NoBody))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":200,"message":"Healthy"}`, w.Body.String())
}
