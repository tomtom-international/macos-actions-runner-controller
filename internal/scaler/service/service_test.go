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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

// MockASGClient is a mock implementation of ASGClient
type MockASGClient struct {
	mock.Mock
}

func (m *MockASGClient) GetCapacity(ctx context.Context, groupName string) (int, error) {
	args := m.Called(ctx, groupName)
	return args.Int(0), args.Error(1)
}

func (m *MockASGClient) SetCapacity(ctx context.Context, groupName string, capacity int) error {
	args := m.Called(ctx, groupName, capacity)
	return args.Error(0)
}

func (m *MockASGClient) GetInstanceIDs(ctx context.Context, groupName string) ([]string, error) {
	args := m.Called(ctx, groupName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// MockSQSClient is a mock implementation of SQSClient
type MockSQSClient struct {
	mock.Mock
}

func (m *MockSQSClient) SendMessage(ctx context.Context, messageBody string) (*sqsTypes.SendMessageOutput, error) {
	args := m.Called(ctx, messageBody)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqsTypes.SendMessageOutput), args.Error(1)
}

func TestNewScalingService(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	mockInstanceCoord := &InstanceCoordinator{}
	mockNodeCoord := &NodeCoordinator{}

	service := &ScalingService{
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: mockInstanceCoord,
		nodeCoordinator:     mockNodeCoord,
	}

	assert.NotNil(t, service)
	assert.NotNil(t, service.asgClient)
	assert.NotNil(t, service.sqsClient)
	assert.NotNil(t, service.instanceCoordinator)
	assert.NotNil(t, service.nodeCoordinator)
}

func TestScalingService_ProcessScalingAction_InputValidation(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	service := &ScalingService{
		asgClient: mockASG,
		sqsClient: mockSQS,
	}

	tests := []struct {
		name            string
		groupName       string
		expectedError   string
		desiredCapacity int
	}{
		{
			name:            "empty group name",
			groupName:       "",
			desiredCapacity: 5,
			expectedError:   "groupName is required",
		},
		{
			name:            "negative capacity",
			groupName:       "my-asg",
			desiredCapacity: -1,
			expectedError:   "desiredCapacity must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := service.ProcessScalingAction(ctx, tt.groupName, tt.desiredCapacity, false)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestScalingService_ProcessScalingAction_NoChange(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	service := &ScalingService{
		asgClient: mockASG,
		sqsClient: mockSQS,
	}

	mockASG.On("GetCapacity", mock.Anything, "my-asg").Return(10, nil)

	ctx := context.Background()
	err := service.ProcessScalingAction(ctx, "my-asg", 10, false)

	require.NoError(t, err)
	mockASG.AssertExpectations(t)
	mockASG.AssertNotCalled(t, "SetCapacity")
}

func TestScalingService_ProcessScalingAction_DryRun(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	service := &ScalingService{
		asgClient: mockASG,
		sqsClient: mockSQS,
	}

	mockASG.On("GetCapacity", mock.Anything, "my-asg").Return(5, nil)

	ctx := context.Background()
	err := service.ProcessScalingAction(ctx, "my-asg", 10, true)

	require.NoError(t, err)
	mockASG.AssertExpectations(t)
	mockASG.AssertNotCalled(t, "SetCapacity")
}

func TestScalingService_ScaleOut(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	service := &ScalingService{
		asgClient: mockASG,
		sqsClient: mockSQS,
	}

	mockASG.On("GetCapacity", mock.Anything, "my-asg").Return(5, nil)
	mockASG.On("SetCapacity", mock.Anything, "my-asg", 10).Return(nil)

	ctx := context.Background()
	err := service.ProcessScalingAction(ctx, "my-asg", 10, false)

	require.NoError(t, err)
	mockASG.AssertExpectations(t)
}

func TestScalingService_ScaleOut_SetCapacityFails(t *testing.T) {
	mockASG := new(MockASGClient)
	mockSQS := new(MockSQSClient)
	service := &ScalingService{
		asgClient: mockASG,
		sqsClient: mockSQS,
	}

	mockASG.On("GetCapacity", mock.Anything, "my-asg").Return(5, nil)
	mockASG.On("SetCapacity", mock.Anything, "my-asg", 10).Return(errors.New("AWS error"))

	ctx := context.Background()
	err := service.ProcessScalingAction(ctx, "my-asg", 10, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS error")
	mockASG.AssertExpectations(t)
}

func TestScalingService_ScaleIn_Success(t *testing.T) {
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
	service := &ScalingService{
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	mockASG.On("GetCapacity", mock.Anything, "my-asg").Return(10, nil)
	mockASG.On("GetInstanceIDs", mock.Anything, "my-asg").Return([]string{"i-111", "i-222"}, nil)
	mockASG.On("SetCapacity", mock.Anything, "my-asg", 8).Return(nil)

	mockEC2.On("GetDedicatedHosts", mock.Anything).Return([]ec2Types.Host{
		{
			State:          ec2Types.AllocationStateAvailable,
			AllocationTime: testTimeOldHost(),
			Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-111")}},
		},
		{
			State:          ec2Types.AllocationStateAvailable,
			AllocationTime: testTimeOldHost(),
			Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-222")}},
		},
	}, nil)

	mockEC2.On("GetInstances", mock.Anything, []string{"i-111"}).Return([]ec2Types.Instance{
		{
			InstanceId:     aws.String("i-111"),
			PrivateDnsName: aws.String("ip-10-0-1-100"),
			State:          &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
		},
	}, nil)
	mockEC2.On("GetInstances", mock.Anything, []string{"i-222"}).Return([]ec2Types.Instance{
		{
			InstanceId:     aws.String("i-222"),
			PrivateDnsName: aws.String("ip-10-0-1-101"),
			State:          &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
		},
	}, nil)

	mockController.On("GetNodes", mock.Anything).Return([]types.Node{
		{
			ID:   utils.UID("node-1"),
			Name: "ip-10-0-1-100",
			Status: types.Status{
				Condition:   types.Condition{Status: types.Ready},
				Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
				Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 10}},
			},
		},
		{
			ID:   utils.UID("node-2"),
			Name: "ip-10-0-1-101",
			Status: types.Status{
				Condition:   types.Condition{Status: types.Ready},
				Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
				Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 10}},
			},
		},
	}, nil)

	mockSQS.On("SendMessage", mock.Anything, mock.Anything).Return(&sqsTypes.SendMessageOutput{
		MessageId: aws.String("msg-123"),
	}, nil)

	ctx := context.Background()
	err := service.ProcessScalingAction(ctx, "my-asg", 8, false)

	require.NoError(t, err)
	mockASG.AssertExpectations(t)
	mockEC2.AssertExpectations(t)
	mockController.AssertExpectations(t)
	mockSQS.AssertExpectations(t)
}

func TestScalingService_ScaleIn_NoInstancesToTerminate(t *testing.T) {
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
	service := &ScalingService{
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	mockASG.On("GetCapacity", mock.Anything, "my-asg").Return(10, nil)
	mockASG.On("GetInstanceIDs", mock.Anything, "my-asg").Return([]string{"i-111"}, nil)

	// No releasable hosts
	mockEC2.On("GetDedicatedHosts", mock.Anything).Return([]ec2Types.Host{}, nil)

	ctx := context.Background()
	err := service.ProcessScalingAction(ctx, "my-asg", 8, false)

	require.NoError(t, err)
	// Should not call SetCapacity or SendMessage
	mockASG.AssertNotCalled(t, "SetCapacity")
	mockSQS.AssertNotCalled(t, "SendMessage")
}

func TestScalingService_ScaleIn_SQSSendFails_RollsBack(t *testing.T) {
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
	service := &ScalingService{
		asgClient:           mockASG,
		sqsClient:           mockSQS,
		instanceCoordinator: instanceCoord,
		nodeCoordinator:     nodeCoord,
	}

	mockASG.On("GetCapacity", mock.Anything, "my-asg").Return(10, nil)
	mockASG.On("GetInstanceIDs", mock.Anything, "my-asg").Return([]string{"i-111"}, nil)
	mockASG.On("SetCapacity", mock.Anything, "my-asg", 9).Return(nil).Once()
	// Rollback
	mockASG.On("SetCapacity", mock.Anything, "my-asg", 10).Return(nil).Once()

	mockEC2.On("GetDedicatedHosts", mock.Anything).Return([]ec2Types.Host{
		{
			State:          ec2Types.AllocationStateAvailable,
			AllocationTime: testTimeOldHost(),
			Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-111")}},
		},
	}, nil)

	mockEC2.On("GetInstances", mock.Anything, []string{"i-111"}).Return([]ec2Types.Instance{
		{
			InstanceId:     aws.String("i-111"),
			PrivateDnsName: aws.String("ip-10-0-1-100"),
			State:          &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
		},
	}, nil)

	mockController.On("GetNodes", mock.Anything).Return([]types.Node{
		{
			ID:   utils.UID("node-1"),
			Name: "ip-10-0-1-100",
			Status: types.Status{
				Condition:   types.Condition{Status: types.Ready},
				Capacity:    types.Resources{Runners: utils.Int32String{IntVal: 10}},
				Allocatable: types.Resources{Runners: utils.Int32String{IntVal: 10}},
			},
		},
	}, nil)

	mockSQS.On("SendMessage", mock.Anything, mock.Anything).Return(nil, errors.New("SQS error"))

	ctx := context.Background()
	err := service.ProcessScalingAction(ctx, "my-asg", 8, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQS error")

	mockASG.AssertExpectations(t)
	mockASG.AssertNumberOfCalls(t, "SetCapacity", 2)
}

// Helper function to create old host time
func testTimeOldHost() *time.Time {
	old := time.Now().Add(-25 * time.Hour)
	return &old
}
