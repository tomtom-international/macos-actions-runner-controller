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

package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHTTPServerConfig(t *testing.T) {
	h := http.NotFoundHandler()

	s := newHTTPServer("8041", h)

	assert.Equal(t, ":8041", s.Addr)
	assert.Equal(t, 15*time.Second, s.ReadTimeout)
	assert.Equal(t, 15*time.Second, s.WriteTimeout)
	assert.Equal(t, 120*time.Second, s.IdleTimeout)
	assert.Equal(t, 1<<20, s.MaxHeaderBytes)
	assert.Zero(t, s.ReadHeaderTimeout)
}

func TestNewHTTPServerServesGivenHandler(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	s := newHTTPServer("8041", h)

	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	assert.Equal(t, http.StatusTeapot, w.Code)
}
