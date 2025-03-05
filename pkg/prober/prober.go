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

package prober

import (
	"fmt"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	githubprobe "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe/github"
	httpprobe "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe/http"
	pt "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/types"
	"time"
)

const maxProbeRetries = 3

type prober struct {
	github githubprobe.Prober
	http   httpprobe.Prober
}

func newProber(githubClient *ghclient.Client) *prober {
	return &prober{
		github: githubprobe.New(githubClient),
		http:   httpprobe.New(),
	}
}

func (pb *prober) probe(probeType pt.ProbeType, target *pt.ProbeTarget) (probe.Result, error) {
	var spec *pt.Probe

	switch probeType {
	case pt.Liveness:
		spec = target.LivenessProbe
	case pt.Startup:
		spec = target.StartupProbe
	default:
		return probe.Failure, fmt.Errorf("unknown probe type: %q", probeType)
	}

	if spec == nil {
		logger.Infof("Probe is nil: probeType - %v", probeType)
		return probe.Success, nil
	}

	result, output, err := pb.runProbeWithRetries(spec, target, maxProbeRetries)
	if err != nil || result != probe.Success {
		if err != nil {
			logger.Debugf("Probe errored. ProbeType: %v, target: %v, tartget id: %v, output: %v", probeType.String(), target.Name, target.ID, output)
		} else {
			logger.Debugf("Probe failed. ProbeType: %v, target: %v, tartget id: %v, output: %v", probeType.String(), target.Name, target.ID, output)
		}
		return probe.Failure, err
	}

	logger.Debugf("Probe succeeded. ProbeType %v, target %v, tartget id %v", probeType.String(), target.Name, target.ID)
	return probe.Success, nil
}

func (pb *prober) runProbeWithRetries(spec *pt.Probe, target *pt.ProbeTarget, retries int) (probe.Result, string, error) {
	var err error
	var result probe.Result
	var output string
	for i := 0; i < retries; i++ {
		result, output, err = pb.runProbe(spec, target)
		if err == nil {
			return result, output, nil
		}
	}
	return result, output, err
}

func (pb *prober) runProbe(spec *pt.Probe, target *pt.ProbeTarget) (probe.Result, string, error) {
	timeout := time.Duration(spec.TimeoutSeconds) * time.Second
	switch {
	case spec.HTTPGet != nil:
		logger.Infof("HTTPGet probe for traget: %v with ID: %v", target.Name, target.ID)
		return pb.http.Probe(timeout)

	case spec.GitHubRunnerGet != nil:
		return pb.github.Probe(target.Name)
	default:
		logger.Infof("Failed to find probe builder for container")
		return probe.Unknown, "", fmt.Errorf("missing probe handler")
	}
}
