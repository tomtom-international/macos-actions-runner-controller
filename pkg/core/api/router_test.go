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
	"testing"

	"github.com/stretchr/testify/assert"
)

func newEchoRouter() *Router {
	r := NewRouter()
	r.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Id", req.PathValue("id"))
	})
	r.HandleFunc("GET /items/{id}/tags", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Tags-Id", req.PathValue("id"))
	})
	return r
}

func routerServe(h http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, http.NoBody))
	return w
}

func TestRouterDecodesPathValues(t *testing.T) {
	w := routerServe(newEchoRouter(), http.MethodGet, "/items/a%20b")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "a b", w.Header().Get("X-Id"))
}

func TestRouterEncodedSlashSplitsSegments(t *testing.T) {
	cases := []struct {
		target     string
		wantTagsID string
		wantStatus int
	}{
		{"/items/a%2Fb", "", http.StatusNotFound},
		{"/items/a%2Ftags", "a", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			w := routerServe(newEchoRouter(), http.MethodGet, tc.target)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantTagsID, w.Header().Get("X-Tags-Id"))
		})
	}
}

func TestRouterLeavesCallerRequestUntouched(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/items/a%2Fb", http.NoBody)

	newEchoRouter().ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "/items/a%2Fb", req.URL.RawPath)
}

func TestRouterRedirectsUncleanPath(t *testing.T) {
	cases := []struct {
		method       string
		target       string
		wantLocation string
	}{
		{http.MethodGet, "/items//a", "/items/a"},
		{http.MethodPut, "/items//a", "/items/a"},
		{http.MethodPost, "//items/a/../b", "/items/b"},
		{http.MethodGet, "/items//a?x=1", "/items/a?x=1"},
		{http.MethodGet, "/items/a%2F%2Fb", "/items/a/b"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			w := routerServe(newEchoRouter(), tc.method, tc.target)

			assert.Equal(t, http.StatusMovedPermanently, w.Code)
			assert.Equal(t, tc.wantLocation, w.Header().Get("Location"))
			assert.Empty(t, w.Body.String())
		})
	}
}
