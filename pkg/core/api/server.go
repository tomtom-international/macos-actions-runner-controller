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
	"bufio"
	"errors"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

const maxHeaderBytes = 1 << 20

// NewHTTPServer builds an http.Server with the given timeouts and a 1 MiB header limit.
func NewHTTPServer(addr string, h http.Handler, read, write, idle time.Duration) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        h,
		ReadTimeout:    read,
		WriteTimeout:   write,
		IdleTimeout:    idle,
		MaxHeaderBytes: maxHeaderBytes,
	}
}

// Middleware logs every request at Debug level and turns a handler panic into a 500.
func Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			logger.Debugf("%s %s %d %s", r.Method, r.URL.Path, sw.code(), time.Since(start))
		}()
		defer recoverPanic(sw, r)
		h.ServeHTTP(sw, r)
	})
}

func recoverPanic(sw *statusWriter, r *http.Request) {
	p := recover()
	if p == nil {
		return
	}
	if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		panic(p)
	}
	logger.Errorf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, p, debug.Stack())
	if sw.status != 0 {
		panic(http.ErrAbortHandler)
	}
	http.Error(sw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) code() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil && w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
