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

package ec2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	defaultRetryMaxAttempts = 3
	awsConfigLoadTimeout    = 30 * time.Second
)

type EC2Client struct {
	client *ec2.Client
}

type ClientConfig struct {
	Region           string
	RetryMaxAttempts int
}

// NewClient creates a new EC2Client with the provided configuration.
// It loads AWS credentials from the environment and establishes a connection.
// Returns an error if the configuration is invalid or AWS credentials cannot be loaded.
func NewClient(ec2Cfg ClientConfig) (*EC2Client, error) {
	if ec2Cfg.Region == "" {
		return nil, errors.New("region is required")
	}
	if ec2Cfg.RetryMaxAttempts == 0 {
		ec2Cfg.RetryMaxAttempts = defaultRetryMaxAttempts
	}
	ctx, cancel := context.WithTimeout(context.Background(), awsConfigLoadTimeout)
	defer cancel()
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(ec2Cfg.Region),
		config.WithRetryMaxAttempts(ec2Cfg.RetryMaxAttempts),
		config.WithRetryMode(aws.RetryModeStandard),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	c := ec2.NewFromConfig(awsCfg)

	return &EC2Client{
		client: c,
	}, nil
}

// GetDedicatedHosts retrieves all dedicated hosts using pagination
func (c *EC2Client) GetDedicatedHosts(ctx context.Context) ([]types.Host, error) {
	var allHosts []types.Host

	describeInput := &ec2.DescribeHostsInput{}
	paginator := ec2.NewDescribeHostsPaginator(c.client, describeInput)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe EC2 dedicated hosts: %w", err)
		}
		allHosts = append(allHosts, page.Hosts...)
	}

	return allHosts, nil
}

// GetInstances retrieves EC2 instances by provided EC2 instance IDs using pagination
func (c *EC2Client) GetInstances(ctx context.Context, instanceIDs []string) ([]types.Instance, error) {
	if len(instanceIDs) == 0 {
		return []types.Instance{}, nil
	}
	var allInstances []types.Instance

	input := &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	}
	paginator := ec2.NewDescribeInstancesPaginator(c.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe EC2 instances: %w", err)
		}
		for _, reservation := range page.Reservations {
			allInstances = append(allInstances, reservation.Instances...)
		}
	}

	return allInstances, nil
}

func (c *EC2Client) TerminateInstances(ctx context.Context, instanceIDs []string) error {
	if len(instanceIDs) == 0 {
		return nil
	}

	input := &ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	}

	_, err := c.client.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate EC2 instances: %w", err)
	}

	return nil
}
