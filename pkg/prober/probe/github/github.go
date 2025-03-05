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
