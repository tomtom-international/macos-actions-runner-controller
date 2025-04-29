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

// Package github provides functionality for interacting with GitHub API
// specifically focused on managing GitHub Actions runners.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v61/github"
)

// GitHubClient defines the interface for GitHub API operations
// related to GitHub Actions runners.
type GitHubClient interface {
	// GetRunnerRegistrationToken retrieves a registration token for GitHub Actions runners.
	GetRunnerRegistrationToken(ctx context.Context) (*github.RegistrationToken, error)

	// GetRunnerByName retrieves GitHub runners by name.
	GetRunnerByName(ctx context.Context, runnerName string) (*github.Runners, error)

	// RemoveRunner removes a GitHub Actions runner by its ID.
	RemoveRunner(ctx context.Context, runnerID int64) error

	// Close cleans up any resources used by the client.
	Close() error
}

// Client implements GitHubClient interface for interacting with GitHub API.
type Client struct {
	ghClient   *github.Client
	restClient *http.Client
	config     ClientConfig // Immutable GitHub configuration
}

// ClientConfig contains the configuration needed to authenticate
// and interact with GitHub API.
type ClientConfig struct {
	// PrivateKeyFile is the path to the private key file
	PrivateKeyFile string
	// Organization is the GitHub organization name
	Organization string
	// PrivateKey contains the private key bytes for the GitHub App
	PrivateKey []byte
	// AppID is the GitHub App ID
	AppID int64
	// InstallationID is the GitHub App Installation ID
	InstallationID int64
}

// New creates a new GitHub client with the provided configuration.
// It returns an error if the client cannot be initialized.
func New(conf ClientConfig) (*Client, error) {
	// Validate required configuration
	if conf.AppID == 0 {
		return nil, fmt.Errorf("AppID is required")
	}
	if conf.InstallationID == 0 {
		return nil, fmt.Errorf("InstallationID is required")
	}
	if conf.Organization == "" {
		return nil, fmt.Errorf("Organization is required")
	}
	if len(conf.PrivateKey) == 0 && conf.PrivateKeyFile == "" {
		return nil, fmt.Errorf("either PrivateKey or PrivateKeyFile must be provided")
	}

	var itr *ghinstallation.Transport
	var err error

	if len(conf.PrivateKey) > 0 {
		itr, err = ghinstallation.New(http.DefaultTransport, conf.AppID, conf.InstallationID, conf.PrivateKey)
	} else {
		itr, err = ghinstallation.NewKeyFromFile(http.DefaultTransport, conf.AppID, conf.InstallationID, conf.PrivateKeyFile)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub transport: %w", err)
	}

	ghClient := github.NewClient(&http.Client{Transport: itr})
	restClient := &http.Client{Transport: itr}

	return &Client{
		ghClient:   ghClient,
		restClient: restClient,
		config:     conf,
	}, nil
}

// GetRunnerRegistrationToken retrieves a registration token for GitHub Actions runners
// in the configured organization.
func (c *Client) GetRunnerRegistrationToken() (*github.RegistrationToken, error) {
	ctx := context.Background()
	token, _, err := c.ghClient.Actions.CreateOrganizationRegistrationToken(ctx, c.config.Organization)

	if err != nil {
		return nil, fmt.Errorf("failed to create organization registration token: %w", err)
	}

	return token, nil
}

// GetRunnerByName retrieves GitHub runners that match the provided name
// in the configured organization.
func (c *Client) GetRunnerByName(runnerName string) (*github.Runners, error) {
	ctx := context.Background()
	if runnerName == "" {
		return nil, fmt.Errorf("runner name cannot be empty")
	}

	// GitHub client does not have a method to get a runner by name, so we use the REST API
	githubUrl := fmt.Sprintf(
		"https://api.github.com/orgs/%v/actions/runners?name=%v",
		url.QueryEscape(c.config.Organization),
		url.QueryEscape(runnerName),
	)
	req, err := http.NewRequestWithContext(ctx, "GET", githubUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for runner %q: %w", runnerName, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.restClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to execute request for runner %q: %w", runnerName, err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to get GitHub runner by name %q: status code %d",
			runnerName, res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body for runner %q: %w", runnerName, err)
	}

	runners := &github.Runners{}
	err = json.Unmarshal(body, runners)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal runners response for %q: %w", runnerName, err)
	}

	return runners, nil
}

// RemoveRunner removes a GitHub Actions runner by its ID from the configured organization.
func (c *Client) RemoveRunner(runnerID int64) error {
	ctx := context.Background()
	_, err := c.ghClient.Actions.RemoveOrganizationRunner(ctx, c.config.Organization, runnerID)
	if err != nil {
		return fmt.Errorf("failed to remove runner %d: %w", runnerID, err)
	}
	return nil
}

// Close cleans up any resources used by the client.
// Currently, this is a no-op as there are no resources that need explicit cleanup,
// but the method is provided for interface compatibility and future-proofing.
func (c *Client) Close() error {
	// Currently a no-op as there are no resources that need explicit cleanup
	return nil
}
