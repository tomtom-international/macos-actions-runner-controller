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

package types

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type Probe struct {
	ProbeHandler `json:",inline" yaml:",inline"`

	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty" yaml:"initialDelaySeconds,omitempty"`

	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`

	PeriodSeconds int32 `json:"periodSeconds,omitempty" yaml:"periodSeconds,omitempty"`

	SuccessThreshold int32 `json:"successThreshold,omitempty"  yaml:"successThreshold,omitempty"`

	FailureThreshold int32 `json:"failureThreshold,omitempty" yaml:"failureThreshold,omitempty"`
}

type ProbeHandler struct {
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty" yaml:"httpGet,omitempty"`

	GitHubRunnerGet *GitHubRunnerGetAction `json:"githubRunnerGet,omitempty" yaml:"githubRunnerGet,omitempty"`

	TartVMStatusGet *TartVMStatusGetAction `json:"tartVMStatusGet,omitempty" yaml:"tartVMStatusGet,omitempty"`
}

// URIScheme identifies the scheme used for connection to a host for Get actions
// +enum
type URIScheme string

const (
	// URISchemeHTTP means that the scheme used will be http://
	URISchemeHTTP URIScheme = "HTTP"
	// URISchemeHTTPS means that the scheme used will be https://
	URISchemeHTTPS URIScheme = "HTTPS"
)

// HTTPHeader describes a custom header to be used in HTTP probes
type HTTPHeader struct {
	// The header field name.
	// This will be canonicalized upon output, so case-variant names will be understood as the same header.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// The header field value
	Value string `json:"value" protobuf:"bytes,2,opt,name=value"`
}

// HTTPGetAction describes an action based on HTTP Get requests.
type HTTPGetAction struct {
	// Path to access on the HTTP server.
	// +optional
	Path string `json:"path,omitempty"`
	// Host name to connect to, defaults to the pod IP. You probably want to set
	// "Host" in httpHeaders instead.
	// +optional
	Host string `json:"host,omitempty"`
	// Scheme to use for connecting to the host.
	// Defaults to HTTP.
	// +optional
	Scheme URIScheme `json:"scheme,omitempty"`
	// Custom headers to set in the request. HTTP allows repeated headers.
	// +optional
	// +listType=atomic
	HTTPHeaders []HTTPHeader `json:"httpHeaders,omitempty"`
	// Name or number of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	Port utils.Int32String `json:"port"`
}

type GitHubRunnerGetAction struct {
	RunnerName string `json:"runnerName,omitempty" yaml:"runnerName,omitempty"`
}

type TartVMStatusGetAction struct {
}

type ProbeTarget struct {
	StartupProbe  *Probe          `json:"startupProbe,omitempty"`
	LivenessProbe *Probe          `json:"livenessProbe,omitempty"`
	ID            utils.UID       `json:"id"`
	TartVMName    string          `json:"tartVMName"`
	Name          string          `json:"name"`
	Type          ProbeTargetType `json:"type"`
}

type ProbeTargetType string

const (
	// ProbeTargetTypeTartRunner means that the target of probe is a TartRunner
	ProbeTargetTypeTartRunner ProbeTargetType = "runner"
	// ProbeTargetTypeTarter means that the target of probe is a Tarter application.
	// Used by TarterController Application to probe the every Tarter.
	ProbeTargetTypeTarter ProbeTargetType = "tarter"
)

type ProbeType int

const (
	Liveness ProbeType = iota
	Startup
)

func (pr ProbeType) String() string {
	switch pr {
	case Liveness:
		return "Liveness"
	case Startup:
		return "Startup"
	default:
		return "UNKNOWN"
	}
}
