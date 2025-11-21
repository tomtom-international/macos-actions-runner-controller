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
	"slices"
	"time"

	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/ec2"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

const defaultMinAllocationHours = 24 * time.Hour

type EC2Client interface {
	GetInstances(ctx context.Context, instanceIDs []string) ([]ec2Types.Instance, error)
	GetDedicatedHosts(ctx context.Context) ([]ec2Types.Host, error)
	TerminateInstances(ctx context.Context, instanceIDs []string) error
}

type InstanceCoordinatorConfig struct {
	MinAllocationHours time.Duration
}

type InstanceCoordinator struct {
	ec2Client EC2Client
	cfg       InstanceCoordinatorConfig
}

func NewInstanceCoordinator(cfg InstanceCoordinatorConfig, ec2Client *ec2.EC2Client) *InstanceCoordinator {
	if cfg.MinAllocationHours == 0 {
		cfg.MinAllocationHours = defaultMinAllocationHours
	}

	return &InstanceCoordinator{
		cfg:       cfg,
		ec2Client: ec2Client,
	}
}

// SelectForTermination determine which instances can be safely terminated.
// Retrieve EC2 instances running on dedicated hosts that allocated more than 24h
// and could be terminated with releasing dedicate host.
func (c *InstanceCoordinator) SelectForTermination(
	ctx context.Context,
	instanceIDs []string,
	instancesToTerminate int,
) ([]string, error) {

	dedicatedHosts, err := c.getReleasableDedicatedHosts(ctx)
	if err != nil {
		logger.Errorf("Failed to get releasable dedicated hosts: %v", err)
		return nil, err
	}

	if len(dedicatedHosts) == 0 {
		logger.Infof("No releasable dedicated hosts found")
		return nil, nil
	}

	asgInstanceSet := make(map[string]bool, len(instanceIDs))
	for _, id := range instanceIDs {
		asgInstanceSet[id] = true
	}

	var selectedInstanceIDs []string

	for _, host := range dedicatedHosts {
		if len(host.Instances) > 0 {
			if host.Instances[0].InstanceId != nil {
				instID := *host.Instances[0].InstanceId
				if asgInstanceSet[instID] {
					selectedInstanceIDs = append(selectedInstanceIDs, instID)
				}
			}
		}
	}

	// Limit to the number of instances we need to terminate
	if len(selectedInstanceIDs) > instancesToTerminate {
		selectedInstanceIDs = selectedInstanceIDs[:instancesToTerminate]
	}

	return selectedInstanceIDs, nil
}

// GetActiveInstancesInfo retrieves EC2 instance information for the provided instance IDs.
func (c *InstanceCoordinator) GetActiveInstancesInfo(ctx context.Context, instanceIDs []string) ([]ec2Types.Instance, error) {
	if len(instanceIDs) == 0 {
		logger.Warnf("No instance IDs provided to GetInstancesInfo")
		return nil, nil
	}

	// The API does not support partial success for non-existent instance IDs
	// So we need to send describe instance request for each instance and handle errors gracefully
	var instances []ec2Types.Instance
	for _, id := range instanceIDs {
		instance, err := c.ec2Client.GetInstances(ctx, []string{id})
		if err != nil {
			logger.Warnf("Failed to get instance info for instance id %s: %v", id, err)
			continue
		}
		if len(instance) == 0 {
			logger.Warnf("Instance %s not found", id)
			continue
		}

		state := instance[0].State.Name
		if state != ec2Types.InstanceStateNameTerminated &&
			state != ec2Types.InstanceStateNameShuttingDown {
			instances = append(instances, instance[0])
		} else {
			logger.Infof("Instance %s already in state %s, skipping", id, state)
		}
	}
	return instances, nil
}

// TerminateInstance terminates the specified EC2 instance.
func (c *InstanceCoordinator) TerminateInstance(ctx context.Context, instanceID string) error {
	err := c.ec2Client.TerminateInstances(ctx, []string{instanceID})
	if err != nil {
		return err
	}
	return nil
}

// getReleasableDedicatedHosts retrieves dedicated hosts that are in 'available' state
// and have been allocated for more than 24 hours with instances on them.
// The returned list is sorted by allocation time in ascending order.
func (c *InstanceCoordinator) getReleasableDedicatedHosts(ctx context.Context) ([]ec2Types.Host, error) {
	allHosts, err := c.ec2Client.GetDedicatedHosts(ctx)
	if err != nil {
		return nil, err
	}

	var releasableHosts []ec2Types.Host

	for _, host := range allHosts {
		if host.State == ec2Types.AllocationStateAvailable &&
			host.AllocationTime != nil &&
			time.Since(*host.AllocationTime).Hours() > c.cfg.MinAllocationHours.Hours() &&
			len(host.Instances) != 0 {
			releasableHosts = append(releasableHosts, host)
		}
	}

	// Sort by allocation time
	slices.SortFunc(
		releasableHosts, func(a, b ec2Types.Host) int {
			if a.AllocationTime == nil || b.AllocationTime == nil {
				return 0
			}
			return a.AllocationTime.Compare(*b.AllocationTime)
		},
	)

	return releasableHosts, nil
}
