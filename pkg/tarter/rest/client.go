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

package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/rest"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
)

type Client struct {
	restClient *rest.RESTClient
}

func NewClient(host string) (*Client, error) {
	httpClient := &http.Client{}
	restClient, err := rest.NewRESTClient(host, "", "", httpClient)

	if err != nil {
		return nil, err
	}
	return &Client{
		restClient: restClient,
	}, nil
}

func (c *Client) GetActiveRunners(ctx context.Context) ([]types.RunnerStateList, error) {
	return c.getRunners(ctx, true)
}

func (c *Client) GetAllRunners(ctx context.Context) ([]types.RunnerStateList, error) {
	return c.getRunners(ctx, false)
}

func (c *Client) getRunners(ctx context.Context, activeOnly bool) ([]types.RunnerStateList, error) {
	var statusQuery string
	if !activeOnly {
		statusQuery = "?status=all"
	}
	response := c.restClient.Get().
		SubPath(fmt.Sprintf("/runners%s", statusQuery)).
		Do(ctx)

	if response.Err != nil {
		return nil, &TarterErrors{
			Reason:  getStatusReason(response.StatusCode),
			Message: response.Err.Error(),
		}
	}

	var runners []types.RunnerStateList

	if err := json.Unmarshal(response.Body, &runners); err != nil {
		return nil, &TarterErrors{
			Reason:  getStatusReason(response.StatusCode),
			Message: fmt.Errorf("failed to read runner info response: %v", err).Error(),
		}
	}

	return runners, nil
}
