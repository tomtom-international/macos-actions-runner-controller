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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/rest"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/config"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"net/http"
)

type Client struct {
	restClient *rest.RESTClient
}

func NewClient(config config.ControllerConfig) (*Client, error) {
	httpClient := &http.Client{}
	restClient, err := rest.NewRESTClient(config.Server, config.ApiVersionPath, "", httpClient)

	if err != nil {
		return nil, err
	}
	return &Client{
		restClient: restClient,
	}, nil
}

func (c *Client) RegisterNode(ctx context.Context, node types.Node) (*types.Node, error) {
	payload, err := json.Marshal(types.NodeRegistrationRequest{
		NodeName: node.Name,
		NodeInfo: node.NodeInfo,
		Capacity: node.Status.Capacity,
	})
	if err != nil {
		logger.Errorf("Failed to marshal node into NodeRegistrationRequest: %v", err)
		return nil, &ControllerErrors{
			Reason:  StatusReasonUnknown,
			Message: err.Error(),
		}
	}
	response := c.restClient.Post().
		SubPath("/nodes/register").
		Body(payload).
		Do(ctx)

	if response.Err != nil {
		if response.StatusCode != http.StatusConflict {
			return nil, &ControllerErrors{
				Reason:  getStatusReason(response.StatusCode),
				Message: response.Err.Error(),
			}
		}
	}

	var registrationResponse types.NodeRegistrationResponse
	if err := json.Unmarshal(response.Body, &registrationResponse); err != nil {
		logger.Errorf("Failed to read response: %v", err)
		return nil, &ControllerErrors{
			Reason:  StatusReasonUnknown,
			Message: response.Err.Error(),
		}
	}

	if response.StatusCode == http.StatusConflict {
		return &registrationResponse.Node, &ControllerErrors{
			Reason:  StatusReasonAlreadyExists,
			Code:    string(registrationResponse.RegistrationStatus),
			Message: response.Err.Error(),
		}
	}

	return &registrationResponse.Node, nil
}

func (c *Client) SendHeartbeat(ctx context.Context, node types.Node) (*types.Node, error) {
	payload, err := json.Marshal(types.NodeHeartbeatRequest{
		NodeName:    node.Name,
		Allocatable: node.Status.Allocatable,
		Status:      node.Status.Condition.Status,
	})
	if err != nil {
		logger.Errorf("Failed to marshal node into NodeHeartbeatRequest: %v", err)
		return nil, &ControllerErrors{
			Reason:  StatusReasonUnknown,
			Message: err.Error(),
		}
	}
	response := c.restClient.Put().
		SubPath(fmt.Sprintf("/nodes/%s/status", node.ID)).
		Body(payload).
		Do(ctx)

	if response.Err != nil {
		return nil, &ControllerErrors{
			Reason:  getStatusReason(response.StatusCode),
			Message: response.Err.Error(),
		}
	}

	var responseNode types.Node
	if err := json.Unmarshal(response.Body, &responseNode); err != nil {
		logger.Errorf("Failed to read heartbeat response: %v", err)
		return nil, &ControllerErrors{
			Reason:  getStatusReason(response.StatusCode),
			Message: err.Error(),
		}
	}

	return &responseNode, nil
}

func (c *Client) GetRunnerInfo(ctx context.Context, runnerID utils.UID) (*types.Runner, error) {
	response := c.restClient.Get().
		SubPath(fmt.Sprintf("/runners/%s", runnerID)).
		Do(ctx)

	if response.Err != nil {
		return nil, &ControllerErrors{
			Reason:  getStatusReason(response.StatusCode),
			Message: response.Err.Error(),
		}
	}

	var runner types.Runner
	if err := json.Unmarshal(response.Body, &runner); err != nil {
		logger.Errorf("Failed to read runner info response: %v", err)
		return nil, &ControllerErrors{
			Reason:  getStatusReason(response.StatusCode),
			Message: err.Error(),
		}
	}

	return &runner, nil
}

func (c *Client) UpdateRunnerStatus(ctx context.Context, update types.RunnerStatusUpdate) error {
	payload, err := json.Marshal(update)
	if err != nil {
		logger.Errorf("Failed to marshal runner status into RunnerStatusUpdate: %v", err)
		return &ControllerErrors{
			Reason:  StatusReasonUnknown,
			Message: err.Error(),
		}
	}
	response := c.restClient.Put().
		SubPath(fmt.Sprintf("/runners/%s/status", update.ID)).
		Body(payload).
		Do(ctx)

	if response.Err != nil {
		return &ControllerErrors{
			Reason:  getStatusReason(response.StatusCode),
			Message: response.Err.Error(),
		}
	}

	return nil
}
