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
	"time"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/asg"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/sqs"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

const (
	defaultTerminationRetryCount     = 12
	defaultTerminationTimeoutMinutes = 5 * time.Minute
)

type TerminationServiceConfig struct {
	TerminationTimeoutMinutes time.Duration
	MaxTerminationRetries     int
}

type TerminationService struct {
	asgClient           ASGClient
	sqsClient           SQSClient
	instanceCoordinator *InstanceCoordinator
	nodeCoordinator     *NodeCoordinator
	cfg                 TerminationServiceConfig
}

func NewTerminationService(
	cfg TerminationServiceConfig,
	asgClient *asg.ASGClient,
	sqsClient *sqs.SQSClient,
	instanceCoordinator *InstanceCoordinator,
	nodeCoordinator *NodeCoordinator,
) *TerminationService {

	if cfg.TerminationTimeoutMinutes == 0 {
		cfg.TerminationTimeoutMinutes = defaultTerminationTimeoutMinutes
	}
	if cfg.MaxTerminationRetries == 0 {
		cfg.MaxTerminationRetries = defaultTerminationRetryCount
	}
	return &TerminationService{
		cfg:                 cfg,
		asgClient:           asgClient,
		sqsClient:           sqsClient,
		instanceCoordinator: instanceCoordinator,
		nodeCoordinator:     nodeCoordinator,
	}
}

// ProcessNodeTermination processes graceful termination of nodes
//
//gocyclo:ignore
func (t *TerminationService) ProcessNodeTermination(msg *ScalingOperation) error {
	logger.Infof("Processing node termination for node group '%s'...", msg.GroupName)
	ctx, cancel := context.WithTimeout(context.Background(), t.cfg.TerminationTimeoutMinutes)
	defer cancel()

	if msg.Attempts > t.cfg.MaxTerminationRetries {
		logger.Warnf(
			"Max retry count reached for termination nodes in node group %s. Nodes to terminate left: %v",
			msg.GroupName,
			msg.NodesToTerminate,
		)
		t.revertFailedScalingOperation(ctx, msg.GroupName, msg.TargetSize, msg.NodesToTerminate)
		return nil
	}

	if len(msg.NodesToTerminate) == 0 {
		logger.Infof("No nodes to terminate for node group %s", msg.GroupName)
		return nil
	}

	var nodesToTerminate []Node
	var nodesToRetry []Node

	// 1. Try disable node via controller API
	for _, node := range msg.NodesToTerminate {
		logger.Infof("Preparing %s node for termination...", node.Hostname)
		err := t.nodeCoordinator.PrepareNodeForTermination(ctx, &node)
		if err != nil {
			logger.Warnf("Failed preparing node node %s for termination: %v", node.ID, err)
			nodesToRetry = append(nodesToRetry, node)
			continue
		}
		if node.ActiveRunners != 0 {
			node.TerminationState = TerminationStateDraining
			nodesToRetry = append(nodesToRetry, node)
			continue
		}
		node.TerminationState = TerminationStateReadyToTerminate
		nodesToTerminate = append(nodesToTerminate, node)
	}

	if len(nodesToTerminate) > 0 {
		logger.Infof("Terminating %d nodes...", len(nodesToTerminate))
		for _, node := range nodesToTerminate {
			// 2. Terminate instances
			if node.TerminationState == TerminationStateReadyToTerminate {
				logger.Infof("Terminating node %s with instance id %s", node.Hostname, node.InstanceID)
				err := t.instanceCoordinator.TerminateInstance(ctx, node.InstanceID)
				if err != nil {
					logger.Warnf("Failed to terminate instance %s: %v", node.InstanceID, err)
					nodesToRetry = append(nodesToRetry, node)
					continue
				}
				node.TerminationState = TerminationStateTerminating
				logger.Infof("Successfully terminated instance %s", node.InstanceID)
			}

			// 3. Deregister node from controller
			logger.Infof("Deregistering node %s with instance id %s", node.Hostname, node.InstanceID)
			err := t.nodeCoordinator.DeregisterNode(ctx, node.ID)
			if err != nil {
				logger.Warnf("Failed to deregister node %s: %v", node.ID, err)
				nodesToRetry = append(nodesToRetry, node)
				continue
			}
		}
	}

	// 4. Re-queue nodes that need retry
	if len(nodesToRetry) > 0 {
		logger.Infof("Re-queuing %d nodes for retry", len(nodesToRetry))
		err := t.requeueNodesForRetry(ctx, msg.GroupName, nodesToRetry, msg.Attempts+1, msg.TargetSize)
		if err != nil {
			logger.Errorf("Failed to requeue nodes for retry: %v", err)
			return err
		}
	}

	logger.Infof("Node termination for node group %s finished", msg.GroupName)
	return nil
}

func (t *TerminationService) requeueNodesForRetry(
	ctx context.Context,
	groupName string,
	nodes []Node,
	attempts int,
	targetSize int,
) error {

	newMsg := ScalingOperation{
		GroupName:        groupName,
		NodesToTerminate: nodes,
		Attempts:         attempts,
		TargetSize:       targetSize,
	}
	message, err := json.Marshal(newMsg)
	if err != nil {
		logger.Errorf("Failed to marshal nodes for re-queueing: %v", err)
		return err
	}
	_, err = t.sqsClient.SendMessage(ctx, string(message))
	if err != nil {
		logger.Errorf("Failed to re-queue message to sqs: %v", err)
		return err
	}
	return nil
}

func (t *TerminationService) revertFailedScalingOperation(
	ctx context.Context,
	groupName string,
	targetSize int,
	failedNodes []Node,
) {
	// revert nodes to active statue to prevent zombie nodes
	for _, node := range failedNodes {
		// Re-enable disabled nodes
		if node.TerminationState != TerminationStateSelected {
			err := t.nodeCoordinator.EnableNode(ctx, node.ID)
			if err != nil {
				logger.Errorf("Failed to re-enable node %s: %v", node.ID, err)
				logger.Errorf("Manual intervention may be needed to re-enable node %s", node.ID)
			}
		}
	}
	// Scale AGS to include failed nodes
	revertCapacity := targetSize + len(failedNodes)
	logger.Infof("Reverting ASG capacity to %d", revertCapacity)
	err := t.asgClient.SetCapacity(ctx, groupName, revertCapacity)
	if err != nil {
		logger.Errorf("Failed to revert ASG capacity: %v", err)
		logger.Errorf("Manual intervention may be needed to set capacity back to %d", revertCapacity)
	}
}
