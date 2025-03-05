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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v61/github"
)

type Client struct {
	ghClient   *github.Client
	restClient *http.Client
	config     ClientConfig // Immutable GitHub configuration
}

type ClientConfig struct {
	AppID          int64
	InstallationID int64
	PrivateKey     []byte
	Organization   string
}

func New(conf ClientConfig) (*Client, error) {

	itr, err := ghinstallation.New(http.DefaultTransport, conf.AppID, conf.InstallationID, conf.PrivateKey)
	if err != nil {
		return nil, err
	}

	ghClient := github.NewClient(&http.Client{Transport: itr})
	restClient := &http.Client{Transport: itr}

	return &Client{
		ghClient:   ghClient,
		restClient: restClient,
		config:     conf,
	}, nil
}

func (c *Client) GetRunnerRegistrationToken() (*github.RegistrationToken, error) {
	ctx := context.Background()
	token, _, err := c.ghClient.Actions.CreateOrganizationRegistrationToken(ctx, c.config.Organization)

	if err != nil {
		return nil, err
	}

	return token, nil
}

func (c *Client) GetRunnerByName(runnerName string) (*github.Runners, error) {
	githubUrl := fmt.Sprintf(
		"https://api.github.com/orgs/%v/actions/runners?name=%v",
		url.QueryEscape(c.config.Organization),
		url.QueryEscape(runnerName),
	)
	req, err := http.NewRequest("GET", githubUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.restClient.Do(req)

	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		fmt.Printf("Get Runner ID by name <%v> response status code is %v (expected 200), url: %v",
			runnerName, res.StatusCode, res.Request.URL)
		return nil, errors.New("failed to get GigHub Runner by name")
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	runners := &github.Runners{}
	err = json.Unmarshal(body, runners)
	if err != nil {
		return nil, err
	}

	return runners, nil
}

func (c *Client) RemoveRunner(runnerID int64) error {
	ctx := context.Background()
	_, err := c.ghClient.Actions.RemoveOrganizationRunner(ctx, c.config.Organization, runnerID)
	if err != nil {
		return err
	}
	return nil
}
