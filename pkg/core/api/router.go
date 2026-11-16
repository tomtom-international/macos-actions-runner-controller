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
	"path"
)

// Router is an http.ServeMux that matches the decoded, cleaned request path the way gorilla/mux did.
type Router struct {
	*http.ServeMux
}

func NewRouter() *Router {
	return &Router{ServeMux: http.NewServeMux()}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	decoded := *req.URL
	// an encoded slash separates segments, as it did under gorilla/mux
	decoded.RawPath = ""
	if p := cleanPath(decoded.Path); p != decoded.Path {
		decoded.Path = p
		w.Header().Set("Location", decoded.String())
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}
	routed := *req
	routed.URL = &decoded
	r.ServeMux.ServeHTTP(w, &routed)
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}
