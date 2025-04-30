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

package github

import (
	"fmt"

	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
)

func New(githubClient *ghclient.Client) Prober {
	return githubProber{
		client: githubClient,
	}
}

type Prober interface {
	Probe(runnerName string) (probe.Result, string, error)
}

type githubProber struct {
	client *ghclient.Client
}

func (g githubProber) Probe(runnerName string) (probe.Result, string, error) {
	runners, err := g.client.GetRunnerByName(runnerName)
	if err != nil {
		return probe.Failure, fmt.Sprintf("Failed to get runner <%v> from github", runnerName), err
	}

	if len(runners.Runners) == 0 {
		return probe.Failure, fmt.Sprintf("Runner <%v> not registered on github", runnerName), nil
	}

	return probe.Success, "github probe success", nil
}
