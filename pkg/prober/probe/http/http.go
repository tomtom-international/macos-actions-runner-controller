/*
 * Copyright 2025 TomTom N.V.
 * Copyright 2015 The Kubernetes Authors.
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

package http

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	"time"
)

func New() Prober {
	return httpProber{}
}

type Prober interface {
	Probe(timeout time.Duration) (probe.Result, string, error)
}

type httpProber struct{}

func (g httpProber) Probe(timeout time.Duration) (probe.Result, string, error) {
	// TODO: JUST MOPCKING THE PROBE
	return probe.Success, "HTTP probe success", nil
}
