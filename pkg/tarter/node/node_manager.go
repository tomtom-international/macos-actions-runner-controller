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

package node

import (
	"context"
	"fmt"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/state"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"sync"
	"time"
)

type Manager struct {
	lastHeartbeatTime     time.Time
	stateManager          *state.StateManager
	node                  *types.Node
	controllerClient      *controller.Client
	nodeInfo              types.NodeInfo
	nodeCapacity          types.Resources
	nodeStatusUpdateRetry int
	syncNodeStatusMux     sync.Mutex
	registerNode          bool
	registrationCompleted bool
	stopSyncNodeStatus    bool
}

func NewManager(
	stateManager *state.StateManager,
	registerNode bool,
	controllerClient *controller.Client,
	nodeInfo types.NodeInfo,
	nodeCapacity types.Resources,
	nodeStatusUpdateRetry int,
) *Manager {
	return &Manager{
		stateManager:          stateManager,
		registerNode:          registerNode,
		controllerClient:      controllerClient,
		nodeInfo:              nodeInfo,
		nodeCapacity:          nodeCapacity,
		nodeStatusUpdateRetry: nodeStatusUpdateRetry,
	}
}

// registerWithController registers the node with the Tarter Controller.
// Safe to call multiple times, but not concurrently (m.registrationCompleted is
// not locked).
func (m *Manager) registerWithController() {
	if m.registrationCompleted {
		return
	}

	step := 500 * time.Millisecond

	for {
		time.Sleep(step)
		step *= 2
		if step >= 7*time.Second {
			step = 7 * time.Second
		}

		node := m.initialNode()

		logger.Infof("Trying to register node %s", node.Name)
		registered := m.tryRegisterWithController(node)
		if registered {
			logger.Infof("Successfully registered node %s", node.Name)
			m.node = node
			m.registrationCompleted = true
			return
		}
	}
}

func (m *Manager) tryRegisterWithController(node *types.Node) bool {
	registeredNode, err := m.controllerClient.RegisterNode(context.TODO(), *node)
	if registeredNode != nil {
		node.ID = registeredNode.ID
	}
	if err == nil {
		return true
	}

	if !controller.IsAlreadyExists(err) {
		logger.Errorf("Unable to register node %s with Tarter Controller. Error: %v", node.Name, err)
		return false
	}

	if controller.ErrorCode(err) == string(types.NodeDeregistered) {
		logger.Infof("Node %s was deregistered, stop registration process", node.Name)
		m.stopSyncNodeStatus = true
		return false
	}

	logger.Infof("Node %s was previously registered", node.Name)

	// TODO: Check if existingNode registered by Tarter of this node.
	// Controller should have unique information about Tarter that registered the node.

	// TODO: Patch node if it differs from existingNode and send it to Controller.

	return true
}

// SyncNodeStatus called periodically from a goroutine.
// It synchronizes node status to Tarter Controller if there is any change or enough time
// passed from the last sync, registering the Tarter first if necessary.
func (m *Manager) SyncNodeStatus() {
	m.syncNodeStatusMux.Lock()
	defer m.syncNodeStatusMux.Unlock()
	ctx := context.Background()

	// If stopSyncNodeStatus is set, do not sync node status.
	// This is used to stop the periodic sync when the node is deregistered.
	if m.stopSyncNodeStatus {
		return
	}
	if m.controllerClient == nil {
		return
	}
	if m.registerNode {
		// This will exit immediately if it doesn't need to register Tarter.
		m.registerWithController()
	}
	// Send heartbeat with node status to Tarter Controller.
	if err := m.updateNodeStatus(ctx); err != nil {
		logger.Errorf("Unable to update node status, error: %v", err)
	}
}

// SyncNodeStatusOnce synchronizes node status to Tarter Controller once.
func (m *Manager) SyncNodeStatusOnce() {
	m.syncNodeStatusMux.Lock()
	defer m.syncNodeStatusMux.Unlock()

	ctx := context.Background()
	updatedNode, err := m.updateNode(*m.node)
	_, err = m.controllerClient.SendHeartbeat(ctx, updatedNode)
	if err != nil {
		logger.Errorf("Unable to update node status, error: %v", err)
	}
	m.node = &updatedNode
}

// updateNodeStatus updates node status to Tarter Controller with retries if there is any
// change or enough time passed from the last sync.
func (m *Manager) updateNodeStatus(ctx context.Context) error {
	for i := 0; i < m.nodeStatusUpdateRetry; i++ {
		if err := m.sendHeartbeat(ctx, m.node); err != nil {
			logger.Errorf("Error updating node status, will retry. Error: %v", err.Error())
			time.Sleep(100 * time.Millisecond)
		} else {
			return nil
		}
	}
	return fmt.Errorf("update node status exceeds retry count")
}

// sendHeartbeat tries to update node status to Tarter Controller if there is any
// change or enough time passed from the last sync.
func (m *Manager) sendHeartbeat(ctx context.Context, node *types.Node) error {
	updatedNode, err := m.updateNode(*node)

	// TODO: Add heartbeat interval check
	//shouldPatchNodeStatus := time.Since(m.lastHeartbeatTime) >= heartbeatInterval

	responseNode, err := m.controllerClient.SendHeartbeat(ctx, updatedNode)

	if controller.ErrorCode(err) == string(types.NodeDeregistered) {
		logger.Infof("Node %s was deregistered, stop syncing node status", node.Name)
		m.stopSyncNodeStatus = true
		return nil
	}
	// TODO: handle heartbeat response errors
	if err != nil {
		return err
	}
	m.node = &updatedNode
	//m.lastHeartbeatTime = time.Now()

	if responseNode != nil {
		if responseNode.Status.Binding != nil {
			err = m.processBindingRunners(ctx, responseNode.Status.Binding)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// initialNode creates a new Node object with the initial status.
// Safe to call multiple times, as it is not modifying m.node
func (m *Manager) initialNode() *types.Node {
	node := types.Node{}
	node.Name = m.nodeInfo.Address.Hostname
	node.NodeInfo = m.nodeInfo
	node.Status.Capacity = m.nodeCapacity
	node.Status.Condition.Status = types.NotReady
	return &node
}

// updateNode updates node allocatable resources and status.
// Safe to call multiple times, as it is not modifying m.node
func (m *Manager) updateNode(node types.Node) (types.Node, error) {
	// TODO: implement node health check to sent proper status
	allocatableResources := m.node.Status.Capacity
	activeRunners := m.stateManager.GetActiveRunners()
	for _, runner := range activeRunners {
		allocatableResources.CPU.IntVal -= runner.Config.CPU.IntVal
		allocatableResources.Memory.IntVal -= runner.Config.Memory.IntVal
		allocatableResources.Runners.IntVal -= 1
	}
	node.Status.Allocatable = allocatableResources
	node.Status.Condition.Status = types.Ready

	return node, nil
}

func (m *Manager) processBindingRunners(ctx context.Context, runners []types.ResourceBinding) error {
	for _, runner := range runners {
		logger.Debugf("Processing binding runner with id %s", runner.RunnerID)
		runnerInfo, err := m.controllerClient.GetRunnerInfo(ctx, runner.RunnerID)
		if err != nil {
			logger.Errorf("Error getting runner info: %v", err)
			return err
		}

		// checking if runner can fit into node
		if runnerInfo.Config.CPU.IntVal > m.node.Status.Allocatable.CPU.IntVal ||
			runnerInfo.Config.Memory.IntVal > m.node.Status.Allocatable.Memory.IntVal {
			logger.Errorf("Binded runner %s cannot fit into node %s", runner.RunnerID, m.node.Name)
			continue
		}

		m.stateManager.AddRunner(tt.Runner{
			ID:            runner.RunnerID,
			GhaRunnerName: runnerInfo.GhaRunnerName,
			Config:        runnerInfo.Config,
		})
	}
	return nil
}
