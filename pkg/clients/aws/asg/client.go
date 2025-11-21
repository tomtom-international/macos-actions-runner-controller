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

package asg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

const (
	defaultRetryMaxAttempts = 3
	awsConfigLoadTimeout    = 30 * time.Second
)

type ASGClient struct {
	client *autoscaling.Client
}

type ClientConfig struct {
	Region           string
	RetryMaxAttempts int
}

// NewClient creates a new ASGClient with the provided configuration.
// It loads AWS credentials from the environment and establishes a connection.
// Returns an error if the configuration is invalid or AWS credentials cannot be loaded.
func NewClient(asgCfg ClientConfig) (*ASGClient, error) {
	if asgCfg.Region == "" {
		return nil, errors.New("region is required")
	}
	if asgCfg.RetryMaxAttempts == 0 {
		asgCfg.RetryMaxAttempts = defaultRetryMaxAttempts
	}
	ctx, cancel := context.WithTimeout(context.Background(), awsConfigLoadTimeout)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(asgCfg.Region),
		config.WithRetryMaxAttempts(asgCfg.RetryMaxAttempts),
		config.WithRetryMode(aws.RetryModeStandard),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	c := autoscaling.NewFromConfig(cfg)

	return &ASGClient{
		client: c,
	}, nil
}

// getAutoScalingGroup retrieves a single ASG by name
func (c *ASGClient) getAutoScalingGroup(ctx context.Context, groupName string) (*types.AutoScalingGroup, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{groupName},
	}
	result, err := c.client.DescribeAutoScalingGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe auto scaling group: %w", err)
	}
	if len(result.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("auto scaling group '%s' not found", groupName)
	}
	return &result.AutoScalingGroups[0], nil
}

// GetCapacity retrieves current ASG capacity
func (c *ASGClient) GetCapacity(ctx context.Context, groupName string) (int, error) {
	if groupName == "" {
		return 0, errors.New("groupName is required")
	}

	asg, err := c.getAutoScalingGroup(ctx, groupName)
	if err != nil {
		return 0, err
	}

	if asg.DesiredCapacity == nil {
		return 0, fmt.Errorf("auto scaling group '%s' has nil DesiredCapacity", groupName)
	}

	return int(*asg.DesiredCapacity), nil
}

// SetCapacity updates ASG capacity
func (c *ASGClient) SetCapacity(ctx context.Context, groupName string, capacity int) error {
	if groupName == "" {
		return errors.New("groupName is required")
	}
	if capacity < 0 {
		return fmt.Errorf("capacity must be non-negative, got %d", capacity)
	}
	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: &groupName,
		DesiredCapacity:      aws.Int32(int32(capacity)),
	}
	_, err := c.client.SetDesiredCapacity(ctx, input)
	return err
}

// GetInstanceIDs retrieves instance IDs in ASG
func (c *ASGClient) GetInstanceIDs(ctx context.Context, groupName string) ([]string, error) {
	if groupName == "" {
		return nil, errors.New("groupName is required")
	}

	asg, err := c.getAutoScalingGroup(ctx, groupName)
	if err != nil {
		return nil, err
	}

	instanceIDs := make([]string, 0, len(asg.Instances))
	for _, instance := range asg.Instances {
		if instance.InstanceId != nil {
			instanceIDs = append(instanceIDs, *instance.InstanceId)
		}
	}
	return instanceIDs, nil
}
