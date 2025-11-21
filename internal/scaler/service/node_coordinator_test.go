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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

// MockControllerClient is a mock implementation of ControllerClient
type MockControllerClient struct {
	mock.Mock
}

func (m *MockControllerClient) GetNodes(ctx context.Context) ([]types.Node, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Node), args.Error(1)
}

func (m *MockControllerClient) GetNodeInfo(ctx context.Context, nodeID utils.UID) (*types.Node, error) {
	args := m.Called(ctx, nodeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Node), args.Error(1)
}

func (m *MockControllerClient) DisableNode(ctx context.Context, nodeID utils.UID) error {
	args := m.Called(ctx, nodeID)
	return args.Error(0)
}

func (m *MockControllerClient) DeregisterNode(ctx context.Context, nodeID utils.UID) error {
	args := m.Called(ctx, nodeID)
	return args.Error(0)
}

func (m *MockControllerClient) EnableNode(ctx context.Context, nodeID utils.UID) error {
	args := m.Called(ctx, nodeID)
	return args.Error(0)
}

func TestNewNodeCoordinator(t *testing.T) {
	mockController := new(MockControllerClient)
	coordinator := &NodeCoordinator{
		controllerClient: mockController,
	}

	assert.NotNil(t, coordinator)
	assert.NotNil(t, coordinator.controllerClient)
}

func TestNodeCoordinator_GetNodesInfo(t *testing.T) {
	tests := []struct {
		controllerError error
		name            string
		controllerNodes []types.Node
		expectedCount   int
		expectedError   bool
	}{
		{
			name: "successfully get nodes",
			controllerNodes: []types.Node{
				{
					ID:   utils.UID("node-1"),
					Name: "ip-10-0-1-100",
					Status: types.Status{
						Condition: types.Condition{
							Status: types.Ready,
						},
						Capacity: types.Resources{
							Runners: utils.Int32String{IntVal: 10},
						},
						Allocatable: types.Resources{
							Runners: utils.Int32String{IntVal: 8},
						},
					},
				},
				{
					ID:   utils.UID("node-2"),
					Name: "ip-10-0-1-101",
					Status: types.Status{
						Condition: types.Condition{
							Status: types.Disabled,
						},
						Capacity: types.Resources{
							Runners: utils.Int32String{IntVal: 10},
						},
						Allocatable: types.Resources{
							Runners: utils.Int32String{IntVal: 10},
						},
					},
				},
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:            "empty node list",
			controllerNodes: []types.Node{},
			expectedCount:   0,
			expectedError:   false,
		},
		{
			name:            "controller error",
			controllerError: errors.New("controller unavailable"),
			expectedError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := new(MockControllerClient)
			coordinator := &NodeCoordinator{
				controllerClient: mockController,
			}

			mockController.On("GetNodes", mock.Anything).Return(tt.controllerNodes, tt.controllerError)

			ctx := context.Background()
			nodes, err := coordinator.GetNodesInfo(ctx)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, nodes, tt.expectedCount)

				// Verify active runners calculation
				for i, node := range nodes {
					expectedActive := int(tt.controllerNodes[i].Status.Capacity.Runners.IntVal -
						tt.controllerNodes[i].Status.Allocatable.Runners.IntVal)
					assert.Equal(t, expectedActive, node.ActiveRunners)
					assert.Equal(t, tt.controllerNodes[i].ID, node.ID)
					assert.Equal(t, tt.controllerNodes[i].Name, node.Hostname)
					assert.Equal(t, tt.controllerNodes[i].Status.Condition.Status, node.Status)
				}
			}

			mockController.AssertExpectations(t)
		})
	}
}

func TestNodeCoordinator_PrepareNodeForTermination(t *testing.T) {
	tests := []struct {
		name           string
		inputNode      Node
		controllerNode *types.Node
		getNodeError   error
		disableError   error
		expectedStatus types.NodeStatus
		expectedError  bool
		expectDisable  bool
	}{
		{
			name: "disable ready node",
			inputNode: Node{
				ID:       utils.UID("node-1"),
				Hostname: "host-1",
			},
			controllerNode: &types.Node{
				ID:   utils.UID("node-1"),
				Name: "host-1",
				Status: types.Status{
					Condition: types.Condition{
						Status: types.Ready,
					},
					Capacity: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
					Allocatable: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
				},
			},
			expectedStatus: types.Disabled,
			expectedError:  false,
			expectDisable:  true,
		},
		{
			name: "node already disabled",
			inputNode: Node{
				ID:       utils.UID("node-2"),
				Hostname: "host-2",
			},
			controllerNode: &types.Node{
				ID:   utils.UID("node-2"),
				Name: "host-2",
				Status: types.Status{
					Condition: types.Condition{
						Status: types.Disabled,
					},
					Capacity: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
					Allocatable: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
				},
			},
			expectedStatus: types.Disabled,
			expectedError:  false,
			expectDisable:  false,
		},
		{
			name: "node already deregistered",
			inputNode: Node{
				ID:       utils.UID("node-3"),
				Hostname: "host-3",
			},
			controllerNode: &types.Node{
				ID:   utils.UID("node-3"),
				Name: "host-3",
				Status: types.Status{
					Condition: types.Condition{
						Status: types.Deregistered,
					},
					Capacity: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
					Allocatable: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
				},
			},
			expectedStatus: types.Deregistered,
			expectedError:  false,
			expectDisable:  false,
		},
		{
			name: "get node fails",
			inputNode: Node{
				ID:       utils.UID("node-4"),
				Hostname: "host-4",
			},
			getNodeError:  errors.New("node not found"),
			expectedError: true,
		},
		{
			name: "disable fails",
			inputNode: Node{
				ID:       utils.UID("node-5"),
				Hostname: "host-5",
			},
			controllerNode: &types.Node{
				ID:   utils.UID("node-5"),
				Name: "host-5",
				Status: types.Status{
					Condition: types.Condition{
						Status: types.Ready,
					},
					Capacity: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
					Allocatable: types.Resources{
						Runners: utils.Int32String{IntVal: 10},
					},
				},
			},
			disableError:  errors.New("disable failed"),
			expectedError: true,
			expectDisable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := new(MockControllerClient)
			coordinator := &NodeCoordinator{
				controllerClient: mockController,
			}

			mockController.On("GetNodeInfo", mock.Anything, tt.inputNode.ID).
				Return(tt.controllerNode, tt.getNodeError)

			if tt.expectDisable {
				mockController.On("DisableNode", mock.Anything, tt.inputNode.ID).
					Return(tt.disableError)
			}

			ctx := context.Background()
			node := tt.inputNode
			err := coordinator.PrepareNodeForTermination(ctx, &node)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, node.Status)

				// Verify active runners were updated
				if tt.controllerNode != nil {
					expectedActive := int(tt.controllerNode.Status.Capacity.Runners.IntVal -
						tt.controllerNode.Status.Allocatable.Runners.IntVal)
					assert.Equal(t, expectedActive, node.ActiveRunners)
				}
			}

			mockController.AssertExpectations(t)
		})
	}
}

func TestNodeCoordinator_DeregisterNode(t *testing.T) {
	tests := []struct {
		deregisterError error
		name            string
		nodeID          utils.UID
		expectedError   bool
	}{
		{
			name:            "successful deregister",
			nodeID:          utils.UID("node-1"),
			deregisterError: nil,
			expectedError:   false,
		},
		{
			name:            "deregister fails",
			nodeID:          utils.UID("node-2"),
			deregisterError: errors.New("deregister failed"),
			expectedError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := new(MockControllerClient)
			coordinator := &NodeCoordinator{
				controllerClient: mockController,
			}

			mockController.On("DeregisterNode", mock.Anything, tt.nodeID).
				Return(tt.deregisterError)

			ctx := context.Background()
			err := coordinator.DeregisterNode(ctx, tt.nodeID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockController.AssertExpectations(t)
		})
	}
}

func TestNodeCoordinator_EnableNode(t *testing.T) {
	tests := []struct {
		enableError   error
		name          string
		nodeID        utils.UID
		expectedError bool
	}{
		{
			name:          "successful enable",
			nodeID:        utils.UID("node-1"),
			enableError:   nil,
			expectedError: false,
		},
		{
			name:          "enable fails",
			nodeID:        utils.UID("node-2"),
			enableError:   errors.New("enable failed"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := new(MockControllerClient)
			coordinator := &NodeCoordinator{
				controllerClient: mockController,
			}

			mockController.On("EnableNode", mock.Anything, tt.nodeID).
				Return(tt.enableError)

			ctx := context.Background()
			err := coordinator.EnableNode(ctx, tt.nodeID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockController.AssertExpectations(t)
		})
	}
}
