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
	"fmt"

	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/asg"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/sqs"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

type ASGClient interface {
	GetCapacity(ctx context.Context, groupName string) (int, error)
	SetCapacity(ctx context.Context, groupName string, capacity int) error
	GetInstanceIDs(ctx context.Context, groupName string) ([]string, error)
}

type SQSClient interface {
	SendMessage(ctx context.Context, messageBody string) (*sqsTypes.SendMessageOutput, error)
}

// ScalingService handles scaling operations
type ScalingService struct {
	asgClient           ASGClient
	sqsClient           SQSClient
	instanceCoordinator *InstanceCoordinator
	nodeCoordinator     *NodeCoordinator
}

func NewScalingService(
	asgClient *asg.ASGClient,
	sqsClient *sqs.SQSClient,
	instanceCoordinator *InstanceCoordinator,
	nodeCoordinator *NodeCoordinator,
) *ScalingService {

	return &ScalingService{
		asgClient:           asgClient,
		sqsClient:           sqsClient,
		instanceCoordinator: instanceCoordinator,
		nodeCoordinator:     nodeCoordinator,
	}
}

// ProcessScalingAction sets the desired capacity of the node group
//
//gocyclo:ignore
func (s *ScalingService) ProcessScalingAction(
	ctx context.Context,
	groupName string,
	desiredCapacity int,
	dryRun bool,
) error {

	if groupName == "" {
		return fmt.Errorf("groupName is required")
	}
	if desiredCapacity < 0 {
		return fmt.Errorf("desiredCapacity must be non-negative, got %d", desiredCapacity)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	logger.Infof("Processing scaling action for node group '%s', with desired capacity %d",
		groupName,
		desiredCapacity,
	)
	currentCapacity, err := s.asgClient.GetCapacity(ctx, groupName)
	if err != nil {
		logger.Errorf("Failed to get node group current capacity: %v", err)
		return err
	}

	logger.Infof("Current capacity of node group '%s': %d", groupName, currentCapacity)

	if desiredCapacity == currentCapacity {
		logger.Infof("Desired capacity is equal to current capacity of node group '%s'. Skipping...", groupName)
		return nil
	}

	logger.Infof("Setting desired capacity for node group '%s' from %d to %d",
		groupName,
		currentCapacity,
		desiredCapacity,
	)

	if dryRun {
		logger.Infof("Dry run enabled - not performing scaling action")
		return nil
	}

	if desiredCapacity > currentCapacity {
		err = s.scaleOut(ctx, groupName, desiredCapacity)
		if err != nil {
			logger.Errorf("Failed to scale-out node group %s: %v", groupName, err)
			return err
		}
	} else {
		err = s.scaleIn(ctx, groupName, desiredCapacity, currentCapacity)
		if err != nil {
			logger.Errorf("Failed to scale-in node group %s: %v", groupName, err)
			return err
		}
	}
	return nil
}

// scaleOut handles scale-up logic
//
//gocyclo:ignore
func (s *ScalingService) scaleOut(ctx context.Context, groupName string, desiredCapacity int) error {
	logger.Infof("Scaling out %s to %d", groupName, desiredCapacity)
	return s.asgClient.SetCapacity(ctx, groupName, desiredCapacity)
}

// scaleIn handles scale-down logic with graceful shutdown
//
//gocyclo:ignore
func (s *ScalingService) scaleIn(ctx context.Context, groupName string, desiredCapacity int, currentCapacity int) error {
	logger.Infof("Scaling-in node group %s to %d", groupName, desiredCapacity)

	// 1. Get all instances in the ASG
	nodesToTerminate := currentCapacity - desiredCapacity
	asgInstanceIDs, err := s.asgClient.GetInstanceIDs(ctx, groupName)
	if err != nil {
		logger.Errorf("Failed to get node group instances: %v", err)
		return err
	}

	// 2. Determine number of nodes that can be terminated and dedicated hosts can be released
	logger.Debugf("Selecting %d instances for termination from %d ASG instances", nodesToTerminate, len(asgInstanceIDs))
	instanceIDs, err := s.instanceCoordinator.SelectForTermination(ctx, asgInstanceIDs, nodesToTerminate)
	if err != nil {
		logger.Errorf("Failed to select instances for termination: %v", err)
		return err
	}
	logger.Infof("Available %d instances to be terminated for node group %s", len(instanceIDs), groupName)
	if len(instanceIDs) == 0 {
		return nil
	}

	// 3. Get instances info form coordinator
	logger.Debugf("Getting instance info for %d instances", len(instanceIDs))
	ec2Instances, err := s.instanceCoordinator.GetActiveInstancesInfo(ctx, instanceIDs)
	if err != nil {
		return fmt.Errorf("failed to get instances info from coordinator: %w", err)
	}
	logger.Debugf("Received instance info for %d instances", len(ec2Instances))

	// 4. Get nodes info from controller
	logger.Debugf("Reading nodes info from controller")
	nodesInfo, err := s.nodeCoordinator.GetNodesInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get nodes info from controller: %w", err)
	}
	logger.Debugf("Received nodes info for %d instances", len(nodesInfo))

	if len(nodesInfo) == 0 {
		logger.Infof("No nodes registered in controller. Skipping...")
		return nil
	}

	nodesByHostname := make(map[string]*Node, len(nodesInfo))
	for _, i := range nodesInfo {
		nodesByHostname[i.Hostname] = &i
	}

	// 5. Match instances with nodes and prepare termination message
	var nodes []Node
	// Find nodes corresponding to selected instances.
	// Only nodes registered in controller are considered for termination by matching hostname.
	for _, ec2Instance := range ec2Instances {
		hostname := ec2Instance.PrivateDnsName
		if hostname == nil {
			logger.Warnf("Failed to get hostname for instance %s", *ec2Instance.InstanceId)
			continue
		}
		if nodeInfo, ok := nodesByHostname[*hostname]; ok {
			nodes = append(nodes, Node{
				InstanceID:       *ec2Instance.InstanceId,
				Hostname:         nodeInfo.Hostname,
				TerminationState: TerminationStateSelected,
				ID:               nodeInfo.ID,
				Status:           nodeInfo.Status,
				ActiveRunners:    nodeInfo.ActiveRunners,
			})
		}
	}
	if len(nodes) == 0 {
		logger.Infof("No matching nodes found in controller for selected instances. Skipping...")
		return nil
	}

	// 6. Update ASG desired capacity.
	// Actual termination will be handled asynchronously.
	// IMPORTANT: Scale-in protection must be enabled on instances in the ASG to handle graceful termination properly.
	newCapacity := currentCapacity - len(nodes)
	if newCapacity < 0 {
		logger.Warnf("New capacity would be negative (%d), setting to 0", newCapacity)
		newCapacity = 0
	}
	logger.Infof("Setting ASG capacity from %d to %d", currentCapacity, newCapacity)
	err = s.asgClient.SetCapacity(ctx, groupName, newCapacity)
	if err != nil {
		logger.Errorf("Failed to update %s capacity: %v", groupName, err)
		return err
	}

	logger.Infof("Sending %d nodes for termination", len(nodes))
	message, err := json.Marshal(ScalingOperation{
		GroupName:        groupName,
		NodesToTerminate: nodes,
		Attempts:         0,
		OriginalSize:     currentCapacity,
		TargetSize:       desiredCapacity,
	})
	if err != nil {
		logger.Errorf("Failed to marshal ScalingOperation message: %v", err)
		logger.Infof("Reverting ASG capacity to %d", currentCapacity)
		asgErr := s.asgClient.SetCapacity(ctx, groupName, currentCapacity)
		if asgErr != nil {
			logger.Errorf("Failed to revert ASG capacity: %v", asgErr)
		}
		return err
	}
	_, err = s.sqsClient.SendMessage(ctx, string(message))
	if err != nil {
		logger.Errorf("Failed to send ScalingOperation message: %v", err)
		logger.Infof("Reverting ASG capacity to %d", currentCapacity)
		asgErr := s.asgClient.SetCapacity(ctx, groupName, currentCapacity)
		if asgErr != nil {
			logger.Errorf("Failed to revert ASG capacity: %v", asgErr)
		}
		return err
	}
	return nil
}
