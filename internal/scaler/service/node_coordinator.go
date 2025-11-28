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

package service

import (
	"context"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type ControllerClient interface {
	GetNodes(ctx context.Context) ([]types.Node, error)
	GetNodeInfo(ctx context.Context, nodeID utils.UID) (*types.Node, error)
	DisableNode(ctx context.Context, nodeID utils.UID) error
	DeregisterNode(ctx context.Context, nodeID utils.UID) error
	EnableNode(ctx context.Context, nodeID utils.UID) error
}

type NodeCoordinator struct {
	controllerClient ControllerClient
}

func NewNodeCoordinator(controllerClient *controller.Client) *NodeCoordinator {
	return &NodeCoordinator{
		controllerClient: controllerClient,
	}
}

// GetNodesInfo retrieves information about all nodes from the controller.
func (c *NodeCoordinator) GetNodesInfo(ctx context.Context) ([]Node, error) {
	controllerNodes, err := c.controllerClient.GetNodes(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]Node, len(controllerNodes))
	for i, node := range controllerNodes {
		activeRunners := int(node.Status.Capacity.Runners.IntVal - node.Status.Allocatable.Runners.IntVal)
		nodes[i] = Node{
			ID:            node.ID,
			Hostname:      node.Name,
			Status:        node.Status.Condition.Status,
			ActiveRunners: activeRunners,
		}
	}
	return nodes, nil
}

// PrepareNodeForTermination disables the node in the controller to prevent new runners from being assigned.
func (c *NodeCoordinator) PrepareNodeForTermination(ctx context.Context, node *Node) error {
	controllerNode, err := c.controllerClient.GetNodeInfo(ctx, node.ID)
	if err != nil {
		return err
	}
	node.Status = controllerNode.Status.Condition.Status
	node.ActiveRunners = int(controllerNode.Status.Capacity.Runners.IntVal - controllerNode.Status.Allocatable.Runners.IntVal)

	if controllerNode.Status.Condition.Status == types.Disabled ||
		controllerNode.Status.Condition.Status == types.Deregistered {
		logger.Debugf("Node %s already disabled or deregistered", node.ID)
	} else {
		err = c.controllerClient.DisableNode(ctx, node.ID)
		if err != nil {
			return err
		}
		node.Status = types.Disabled
	}

	return nil
}

// DeregisterNode deregisters the node from the controller.
func (c *NodeCoordinator) DeregisterNode(ctx context.Context, nodeID utils.UID) error {
	return c.controllerClient.DeregisterNode(ctx, nodeID)
}

// EnableNode re-enables the node in the controller.
func (c *NodeCoordinator) EnableNode(ctx context.Context, nodeID utils.UID) error {
	return c.controllerClient.EnableNode(ctx, nodeID)
}
