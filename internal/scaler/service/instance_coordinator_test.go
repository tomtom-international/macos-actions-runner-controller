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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEC2Client is a mock implementation of EC2Client
type MockEC2Client struct {
	mock.Mock
}

func (m *MockEC2Client) GetInstances(ctx context.Context, instanceIDs []string) ([]ec2Types.Instance, error) {
	args := m.Called(ctx, instanceIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ec2Types.Instance), args.Error(1)
}

func (m *MockEC2Client) GetDedicatedHosts(ctx context.Context) ([]ec2Types.Host, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ec2Types.Host), args.Error(1)
}

func (m *MockEC2Client) TerminateInstances(ctx context.Context, instanceIDs []string) error {
	args := m.Called(ctx, instanceIDs)
	return args.Error(0)
}

func TestNewInstanceCoordinator(t *testing.T) {
	mockEC2 := new(MockEC2Client)

	tests := []struct {
		name     string
		config   InstanceCoordinatorConfig
		expected time.Duration
	}{
		{
			name: "with custom config",
			config: InstanceCoordinatorConfig{
				MinAllocationHours: 48 * time.Hour,
			},
			expected: 48 * time.Hour,
		},
		{
			name:     "with default config",
			config:   InstanceCoordinatorConfig{},
			expected: defaultMinAllocationHours,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.MinAllocationHours == 0 {
				tt.config.MinAllocationHours = defaultMinAllocationHours
			}
			coordinator := &InstanceCoordinator{
				cfg:       tt.config,
				ec2Client: mockEC2,
			}
			assert.NotNil(t, coordinator)
			assert.Equal(t, tt.expected, coordinator.cfg.MinAllocationHours)
		})
	}
}

func TestInstanceCoordinator_SelectForTermination(t *testing.T) {
	now := time.Now()
	oldHost := now.Add(-25 * time.Hour)

	tests := []struct {
		hostsError           error
		name                 string
		instanceIDs          []string
		hosts                []ec2Types.Host
		expectedIDs          []string
		instancesToTerminate int
		expectedError        bool
	}{
		{
			name:                 "select from available hosts",
			instanceIDs:          []string{"i-111", "i-222", "i-333"},
			instancesToTerminate: 2,
			hosts: []ec2Types.Host{
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &oldHost,
					Instances: []ec2Types.HostInstance{
						{InstanceId: aws.String("i-111")},
					},
				},
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &oldHost,
					Instances: []ec2Types.HostInstance{
						{InstanceId: aws.String("i-222")},
					},
				},
			},
			expectedIDs:   []string{"i-111", "i-222"},
			expectedError: false,
		},
		{
			name:                 "limit selection to requested count",
			instanceIDs:          []string{"i-111", "i-222", "i-333"},
			instancesToTerminate: 1,
			hosts: []ec2Types.Host{
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &oldHost,
					Instances: []ec2Types.HostInstance{
						{InstanceId: aws.String("i-111")},
					},
				},
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &oldHost,
					Instances: []ec2Types.HostInstance{
						{InstanceId: aws.String("i-222")},
					},
				},
			},
			expectedIDs:   []string{"i-111"},
			expectedError: false,
		},
		{
			name:                 "no releasable hosts",
			instanceIDs:          []string{"i-111", "i-222"},
			instancesToTerminate: 2,
			hosts:                []ec2Types.Host{},
			expectedIDs:          nil,
			expectedError:        false,
		},
		{
			name:                 "api error",
			instanceIDs:          []string{"i-111"},
			instancesToTerminate: 1,
			hostsError:           errors.New("API error"),
			expectedError:        true,
		},
		{
			name:                 "filter out non-ASG instances",
			instanceIDs:          []string{"i-111", "i-222"},
			instancesToTerminate: 2,
			hosts: []ec2Types.Host{
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &oldHost,
					Instances: []ec2Types.HostInstance{
						{InstanceId: aws.String("i-111")},
					},
				},
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &oldHost,
					Instances: []ec2Types.HostInstance{
						{InstanceId: aws.String("i-999")}, // Not in ASG
					},
				},
			},
			expectedIDs:   []string{"i-111"},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := new(MockEC2Client)
			coordinator := &InstanceCoordinator{
				cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
				ec2Client: mockEC2,
			}

			mockEC2.On("GetDedicatedHosts", mock.Anything).Return(tt.hosts, tt.hostsError)

			ctx := context.Background()
			result, err := coordinator.SelectForTermination(ctx, tt.instanceIDs, tt.instancesToTerminate)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedIDs, result)
			}

			mockEC2.AssertExpectations(t)
		})
	}
}

func TestInstanceCoordinator_GetActiveInstancesInfo(t *testing.T) {
	tests := []struct {
		mockResponses map[string][]ec2Types.Instance
		mockErrors    map[string]error
		name          string
		instanceIDs   []string
		expectedCount int
	}{
		{
			name:        "all instances active",
			instanceIDs: []string{"i-111", "i-222"},
			mockResponses: map[string][]ec2Types.Instance{
				"i-111": {
					{
						InstanceId: aws.String("i-111"),
						State:      &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
					},
				},
				"i-222": {
					{
						InstanceId: aws.String("i-222"),
						State:      &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name:        "filter terminated instances",
			instanceIDs: []string{"i-111", "i-222", "i-333"},
			mockResponses: map[string][]ec2Types.Instance{
				"i-111": {
					{
						InstanceId: aws.String("i-111"),
						State:      &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
					},
				},
				"i-222": {
					{
						InstanceId: aws.String("i-222"),
						State:      &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameTerminated},
					},
				},
				"i-333": {
					{
						InstanceId: aws.String("i-333"),
						State:      &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameShuttingDown},
					},
				},
			},
			expectedCount: 1, // Only i-111 is active
		},
		{
			name:        "handle API errors gracefully",
			instanceIDs: []string{"i-111", "i-222"},
			mockResponses: map[string][]ec2Types.Instance{
				"i-222": {
					{
						InstanceId: aws.String("i-222"),
						State:      &ec2Types.InstanceState{Name: ec2Types.InstanceStateNameRunning},
					},
				},
			},
			mockErrors: map[string]error{
				"i-111": errors.New("instance not found"),
			},
			expectedCount: 1, // Only i-222 succeeds
		},
		{
			name:          "empty instance list",
			instanceIDs:   []string{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := new(MockEC2Client)
			coordinator := &InstanceCoordinator{
				cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
				ec2Client: mockEC2,
			}

			for _, id := range tt.instanceIDs {
				if resp, ok := tt.mockResponses[id]; ok {
					mockEC2.On("GetInstances", mock.Anything, []string{id}).Return(resp, nil)
				} else if err, ok := tt.mockErrors[id]; ok {
					mockEC2.On("GetInstances", mock.Anything, []string{id}).Return(nil, err)
				}
			}

			ctx := context.Background()
			result, err := coordinator.GetActiveInstancesInfo(ctx, tt.instanceIDs)

			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)

			mockEC2.AssertExpectations(t)
		})
	}
}

func TestInstanceCoordinator_TerminateInstance(t *testing.T) {
	tests := []struct {
		terminateErr  error
		name          string
		instanceID    string
		expectedError bool
	}{
		{
			name:          "successful termination",
			instanceID:    "i-123",
			terminateErr:  nil,
			expectedError: false,
		},
		{
			name:          "termination fails",
			instanceID:    "i-456",
			terminateErr:  errors.New("termination failed"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := new(MockEC2Client)
			coordinator := &InstanceCoordinator{
				cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
				ec2Client: mockEC2,
			}

			mockEC2.On("TerminateInstances", mock.Anything, []string{tt.instanceID}).Return(tt.terminateErr)

			ctx := context.Background()
			err := coordinator.TerminateInstance(ctx, tt.instanceID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockEC2.AssertExpectations(t)
		})
	}
}

func TestInstanceCoordinator_getReleasableDedicatedHosts(t *testing.T) {
	now := time.Now()
	old25h := now.Add(-25 * time.Hour)
	old23h := now.Add(-23 * time.Hour)

	tests := []struct {
		name          string
		description   string
		hosts         []ec2Types.Host
		expectedCount int
	}{
		{
			name: "filter by allocation time and state",
			hosts: []ec2Types.Host{
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &old25h,
					Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-1")}},
				},
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &old23h, // Too young
					Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-2")}},
				},
				{
					State:          ec2Types.AllocationStateUnderAssessment, // Wrong state
					AllocationTime: &old25h,
					Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-3")}},
				},
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: &old25h,
					Instances:      []ec2Types.HostInstance{}, // No instances
				},
			},
			expectedCount: 1, // Only first host qualifies
		},
		{
			name: "sort by allocation time ascending",
			hosts: []ec2Types.Host{
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: aws.Time(now.Add(-26 * time.Hour)),
					Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-1")}},
				},
				{
					State:          ec2Types.AllocationStateAvailable,
					AllocationTime: aws.Time(now.Add(-25 * time.Hour)),
					Instances:      []ec2Types.HostInstance{{InstanceId: aws.String("i-2")}},
				},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := new(MockEC2Client)
			coordinator := &InstanceCoordinator{
				cfg:       InstanceCoordinatorConfig{MinAllocationHours: defaultMinAllocationHours},
				ec2Client: mockEC2,
			}

			mockEC2.On("GetDedicatedHosts", mock.Anything).Return(tt.hosts, nil)

			ctx := context.Background()
			result, err := coordinator.getReleasableDedicatedHosts(ctx)

			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)

			if len(result) > 1 {
				for i := 1; i < len(result); i++ {
					assert.True(t, result[i-1].AllocationTime.Before(*result[i].AllocationTime) ||
						result[i-1].AllocationTime.Equal(*result[i].AllocationTime),
						"hosts should be sorted by allocation time")
				}
			}

			mockEC2.AssertExpectations(t)
		})
	}
}
