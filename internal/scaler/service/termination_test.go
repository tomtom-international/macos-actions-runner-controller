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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

func TestNewTerminationService(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	tests := []struct {
		name               string
		config             TerminationServiceConfig
		expectedTimeout    time.Duration
		expectedMaxRetries int
	}{
		{
			name: "with custom config",
			config: TerminationServiceConfig{
				TerminationTimeoutMinutes: 10 * time.Minute,
				MaxTerminationRetries:     5,
			},
			expectedTimeout:    10 * time.Minute,
			expectedMaxRetries: 5,
		},
		{
			name:               "with defaults",
			config:             TerminationServiceConfig{},
			expectedTimeout:    defaultTerminationTimeoutMinutes,
			expectedMaxRetries: defaultTerminationRetryCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.TerminationTimeoutMinutes == 0 {
				tt.config.TerminationTimeoutMinutes = defaultTerminationTimeoutMinutes
			}
			if tt.config.MaxTerminationRetries == 0 {
				tt.config.MaxTerminationRetries = defaultTerminationRetryCount
			}
			service := &TerminationService{
				cfg:                 tt.config,
				asgClient:           mockASG,
				sqsClient:           mockSQS,
				instanceCoordinator: instanceCoord,
				nodeCoordinator:     nodeCoord,
			}

			assert.NotNil(t, service)
			assert.Equal(t, tt.expectedTimeout, service.cfg.TerminationTimeoutMinutes)
			assert.Equal(t, tt.expectedMaxRetries, service.cfg.MaxTerminationRetries)
		})
	}
}

func TestTerminationService_ProcessNodeTermination_MaxRetriesExceeded(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	cfg := TerminationServiceConfig{
		TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
		MaxTerminationRetries:     3,
	}
	service := &TerminationService{
		cfg:                 cfg,
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 8,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateDraining,
			},
		},
		Attempts: 4, // Exceeds max
	}

	// Should call revert functions
	mockController.On("EnableNode", mock.Anything, utils.UID("node-1")).Return(nil)
	mockASG.On("SetCapacity", mock.Anything, "my-asg", 9).Return(nil) // targetSize + 1 node

	err := service.ProcessNodeTermination(msg)

	require.NoError(t, err)
	mockController.AssertExpectations(t)
	mockASG.AssertExpectations(t)
}

func TestTerminationService_ProcessNodeTermination_EmptyNodeList(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     defaultTerminationRetryCount,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:        "my-asg",
		NodesToTerminate: []Node{},
		Attempts:         0,
	}

	err := service.ProcessNodeTermination(msg)

	require.NoError(t, err)
	// No calls should be made
	mockController.AssertNotCalled(t, "GetNodeInfo")
	mockEC2.AssertNotCalled(t, "TerminateInstances")
}

func TestTerminationService_ProcessNodeTermination_NodeReadyToTerminate(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     defaultTerminationRetryCount,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 9,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateSelected,
			},
		},
		Attempts: 0,
	}

	mockController.On("GetNodeInfo", mock.Anything, utils.UID("node-1")).Return(&types.Node{
		ID:   utils.UID("node-1"),
		Name: "host-1",
		Status: types.Status{
			Condition:   types.Condition{Status: types.Ready},
			Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
			Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 10}}, // All available = idle
		},
	}, nil)

	mockController.On("DisableNode", mock.Anything, utils.UID("node-1")).Return(nil)

	mockEC2.On("TerminateInstances", mock.Anything, []string{"i-111"}).Return(nil)

	mockController.On("DeregisterNode", mock.Anything, utils.UID("node-1")).Return(nil)

	err := service.ProcessNodeTermination(msg)

	require.NoError(t, err)
	mockController.AssertExpectations(t)
	mockEC2.AssertExpectations(t)
	mockSQS.AssertNotCalled(t, "SendMessage")
}

func TestTerminationService_ProcessNodeTermination_NodeBusy_Requeue(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     defaultTerminationRetryCount,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 9,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateSelected,
			},
		},
		Attempts: 0,
	}

	mockController.On("GetNodeInfo", mock.Anything, utils.UID("node-1")).Return(&types.Node{
		ID:   utils.UID("node-1"),
		Name: "host-1",
		Status: types.Status{
			Condition:   types.Condition{Status: types.Disabled},
			Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
			Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 8}}, // 2 runners active
		},
	}, nil)

	mockSQS.On("SendMessage", mock.Anything, mock.MatchedBy(func(body string) bool {
		var requeuedMsg ScalingOperation
		err := json.Unmarshal([]byte(body), &requeuedMsg)
		if err != nil {
			return false
		}
		return requeuedMsg.Attempts == 1 && len(requeuedMsg.NodesToTerminate) == 1
	})).Return(&sqsTypes.SendMessageOutput{MessageId: aws.String("msg-123")}, nil)

	err := service.ProcessNodeTermination(msg)

	require.NoError(t, err)
	mockController.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
	mockEC2.AssertNotCalled(t, "TerminateInstances")
}

func TestTerminationService_ProcessNodeTermination_PrepareNodeFails_Requeue(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     defaultTerminationRetryCount,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 9,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateSelected,
			},
		},
		Attempts: 0,
	}

	mockController.On("GetNodeInfo", mock.Anything, utils.UID("node-1")).
		Return(nil, errors.New("controller unavailable"))

	mockSQS.On("SendMessage", mock.Anything, mock.Anything).
		Return(&sqsTypes.SendMessageOutput{MessageId: aws.String("msg-123")}, nil)

	err := service.ProcessNodeTermination(msg)

	require.NoError(t, err)
	mockController.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

func TestTerminationService_ProcessNodeTermination_TerminationFails_Requeue(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     defaultTerminationRetryCount,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 9,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateSelected,
			},
		},
		Attempts: 0,
	}

	mockController.On("GetNodeInfo", mock.Anything, utils.UID("node-1")).Return(&types.Node{
		ID:   utils.UID("node-1"),
		Name: "host-1",
		Status: types.Status{
			Condition:   types.Condition{Status: types.Ready},
			Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
			Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 10}},
		},
	}, nil)

	mockController.On("DisableNode", mock.Anything, utils.UID("node-1")).Return(nil)

	mockEC2.On("TerminateInstances", mock.Anything, []string{"i-111"}).
		Return(errors.New("AWS error"))

	mockSQS.On("SendMessage", mock.Anything, mock.Anything).
		Return(&sqsTypes.SendMessageOutput{MessageId: aws.String("msg-123")}, nil)

	err := service.ProcessNodeTermination(msg)

	require.NoError(t, err)
	mockController.AssertExpectations(t)
	mockEC2.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

func TestTerminationService_ProcessNodeTermination_DeregisterFails(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     defaultTerminationRetryCount,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 9,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateSelected,
			},
		},
		Attempts: 0,
	}

	mockController.On("GetNodeInfo", mock.Anything, utils.UID("node-1")).Return(&types.Node{
		ID:   utils.UID("node-1"),
		Name: "host-1",
		Status: types.Status{
			Condition:   types.Condition{Status: types.Ready},
			Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
			Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 10}},
		},
	}, nil)

	mockController.On("DisableNode", mock.Anything, utils.UID("node-1")).Return(nil)

	mockEC2.On("TerminateInstances", mock.Anything, []string{"i-111"}).Return(nil)

	mockController.On("DeregisterNode", mock.Anything, utils.UID("node-1")).
		Return(errors.New("deregister failed"))

	mockSQS.On("SendMessage", mock.Anything, mock.Anything).
		Return(&sqsTypes.SendMessageOutput{MessageId: aws.String("msg-123")}, nil)

	err := service.ProcessNodeTermination(msg)

	require.NoError(t, err)
	mockController.AssertExpectations(t)
	mockEC2.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

func TestTerminationService_RequeueFails(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     defaultTerminationRetryCount,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	msg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 9,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateSelected,
			},
		},
		Attempts: 0,
	}

	mockController.On("GetNodeInfo", mock.Anything, utils.UID("node-1")).Return(&types.Node{
		ID:   utils.UID("node-1"),
		Name: "host-1",
		Status: types.Status{
			Condition:   types.Condition{Status: types.Disabled},
			Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
			Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 8}}, // Busy
		},
	}, nil)

	mockSQS.On("SendMessage", mock.Anything, mock.Anything).
		Return(nil, errors.New("SQS error"))

	err := service.ProcessNodeTermination(msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQS error")
}

func TestTerminationService_RevertFailedScalingOperation(t *testing.T) {
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}

	tests := []struct {
		setCapacityError error
		enableErrors     map[string]error
		name             string
		failedNodes      []Node
		targetSize       int
	}{
		{
			name: "successful revert",
			failedNodes: []Node{
				{
					ID:               utils.UID("node-1"),
					TerminationState: TerminationStateDraining,
				},
				{
					ID:               utils.UID("node-2"),
					TerminationState: TerminationStateDraining,
				},
			},
			targetSize:   8,
			enableErrors: map[string]error{},
		},
		{
			name: "enable fails",
			failedNodes: []Node{
				{
					ID:               utils.UID("node-1"),
					TerminationState: TerminationStateDraining,
				},
			},
			targetSize: 9,
			enableErrors: map[string]error{
				"node-1": errors.New("enable failed"),
			},
		},
		{
			name: "skip nodes in selected state",
			failedNodes: []Node{
				{
					ID:               utils.UID("node-1"),
					TerminationState: TerminationStateSelected, // Should skip
				},
				{
					ID:               utils.UID("node-2"),
					TerminationState: TerminationStateDraining, // Should enable
				},
			},
			targetSize:   8,
			enableErrors: map[string]error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := new(MockControllerClient)
			mockASG := new(MockASGClient)
			nodeCoord := &NodeCoordinator{
				controllerClient: mockController,
			}
			service := &TerminationService{
				cfg: TerminationServiceConfig{
					TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
					MaxTerminationRetries:     defaultTerminationRetryCount,
				},
				asgClient:           mockASG,
				sqsClient:           mockSQS,
				instanceCoordinator: instanceCoord,
				nodeCoordinator:     nodeCoord,
			}

			for _, node := range tt.failedNodes {
				if node.TerminationState != TerminationStateSelected {
					if err, ok := tt.enableErrors[string(node.ID)]; ok {
						mockController.On("EnableNode", mock.Anything, node.ID).Return(err)
					} else {
						mockController.On("EnableNode", mock.Anything, node.ID).Return(nil)
					}
				}
			}

			expectedCapacity := tt.targetSize + len(tt.failedNodes)
			mockASG.On("SetCapacity", mock.Anything, "my-asg", expectedCapacity).
				Return(tt.setCapacityError)

			ctx := context.Background()
			service.revertFailedScalingOperation(ctx, "my-asg", tt.targetSize, tt.failedNodes)

			mockController.AssertExpectations(t)
			mockASG.AssertExpectations(t)
		})
	}
}

func TestTerminationService_TargetSizeLostDuringRequeue(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockEC2 := new(MockEC2Client)
	mockController := new(MockControllerClient)

	instanceCoord := &InstanceCoordinator{
		cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
		ec2Client: mockEC2,
	}
	nodeCoord := &NodeCoordinator{
		controllerClient: mockController,
	}

	service := &TerminationService{
		cfg: TerminationServiceConfig{
			TerminationTimeoutMinutes: defaultTerminationTimeoutMinutes,
			MaxTerminationRetries:     0,
		},
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	initialMsg := &ScalingOperation{
		GroupName:  "my-asg",
		TargetSize: 14,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				TerminationState: TerminationStateSelected,
				ActiveRunners:    0,
			},
		},
		Attempts: 0,
	}

	mockController.On("GetNodeInfo", mock.Anything, utils.UID("node-1")).Return(&types.Node{
		ID:   utils.UID("node-1"),
		Name: "host-1",
		Status: types.Status{
			Condition:   types.Condition{Status: types.Disabled},
			Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
			Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 8}},
		},
	}, nil).Once()

	var requeuedMsg1 ScalingOperation
	mockSQS.On("SendMessage", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			body := args.Get(1).(string)
			json.Unmarshal([]byte(body), &requeuedMsg1)
		}).
		Return(&sqsTypes.SendMessageOutput{MessageId: aws.String("msg-1")}, nil).Once()

	err := service.ProcessNodeTermination(initialMsg)
	require.NoError(t, err)

	t.Logf("After attempt 1 - TargetSize in requeued message: %d (expected: 14)", requeuedMsg1.TargetSize)
	t.Logf("After attempt 1 - Attempts in requeued message: %d (expected: 1)", requeuedMsg1.Attempts)

	mockController.On("EnableNode", mock.Anything, utils.UID("node-1")).Return(nil).Once()

	var actualRevertCapacity int
	mockASG.On("SetCapacity", mock.Anything, "my-asg", mock.AnythingOfType("int")).
		Run(func(args mock.Arguments) {
			actualRevertCapacity = args.Get(2).(int)
		}).
		Return(nil).Once()

	err = service.ProcessNodeTermination(&requeuedMsg1)
	require.NoError(t, err)

	expectedRevertCapacity := 15
	t.Logf("Actual revert capacity: %d, Expected: %d", actualRevertCapacity, expectedRevertCapacity)

	assert.Equal(t, expectedRevertCapacity, actualRevertCapacity,
		"BUG: TargetSize was lost during requeue, causing incorrect revert capacity. "+
			"Expected %d but got %d", expectedRevertCapacity, actualRevertCapacity)

	mockController.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
	mockASG.AssertExpectations(t)
}
