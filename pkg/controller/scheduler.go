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

package controller

import (
	"context"
	np "github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/nodepool"
	r "github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/runners"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"sort"
	"time"
)

const schedulerPeriod = 15 * time.Second

type Scheduler struct {
	nodePoolManager *np.Manager
	runnerManager   *r.Manager
}

func newScheduler(nodeManager *np.Manager, runnerManager *r.Manager) *Scheduler {
	return &Scheduler{
		nodePoolManager: nodeManager,
		runnerManager:   runnerManager,
	}
}

func (s *Scheduler) startScheduler(ctx context.Context) {
	logger.Debugf("Starting scheduler")

	ticker := time.NewTicker(schedulerPeriod)
	defer func() {
		logger.Debugf("Stopping scheduler")
		ticker.Stop()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.schedule()
		}
	}
}

// TODO: Explain why 2 pending runners scheduling
// TODO: Handle race condition on controller replications
func (s *Scheduler) schedule() {
	nodes := s.getNodeList()
	if len(nodes) == 0 {
		logger.Debugf("No nodes available for scheduling")
		return
	}
	pendingRunners := s.getPendingRunners()
	if len(pendingRunners) == 0 {
		return
	}

	// Try to assign runners before 2 pending runners failed to assign
	notAllocatedRunners := 0
	for _, runner := range pendingRunners {
		if notAllocatedRunners >= 2 {
			break
		}
		node := s.findNodeSuitsForRunner(runner, nodes)
		if node == nil {
			logger.Debugf("No allocatable capacity for runner <%s> on any node", runner.ID)
			notAllocatedRunners++
			continue
		}
		err := s.assignRunner(runner, node)
		if err != nil {
			logger.Errorf("Failed to assign runner %s to node %s: %v", runner.ID, node.ID, err)
			notAllocatedRunners++
			continue
		}
		logger.Infof("Runner %s assigned to node %s", runner.ID, node.ID)
	}
	logger.Debugf("Scheduling finished")
}

// getPendingRunners returns list of runners with status pending
// sorted by creation timestamp
func (s *Scheduler) getPendingRunners() []*types.Runner {
	runners, err := s.runnerManager.GetRunners()
	if err != nil {
		logger.Errorf("Failed to get runners from db, error: %v", err)
		return nil
	}
	pendingRunners := make([]*types.Runner, 0)
	for _, runner := range runners {
		if runner.Condition.Status == types.Pending {
			pendingRunners = append(pendingRunners, &runner)
		}
	}

	sort.Slice(pendingRunners, func(i, j int) bool {
		return pendingRunners[i].Condition.CreationTimestamp.Before(pendingRunners[j].Condition.CreationTimestamp)
	})

	return pendingRunners
}

// getNodeList returns list of nodes with capacity greater than 0
// and sorted by allocatable runners number from lowest to highest
// to fully pack nodes with proper capacity
func (s *Scheduler) getNodeList() []*types.Node {
	nodes, err := s.nodePoolManager.GetNodePool()
	if err != nil {
		logger.Errorf("Failed to get node list: %v", err)
		return nil
	}
	// Filter out all nodes with Capacity 0
	var filteredNodes []*types.Node
	for _, node := range nodes {
		if node.Status.Allocatable.Runners.IntVal > 0 {
			filteredNodes = append(filteredNodes, &node)
		}
	}

	// Sort nodes by allocatable runners number from lowest to highest
	// to fully pack nodes with proper capacity
	sort.Slice(filteredNodes, func(i, j int) bool {
		return filteredNodes[i].Status.Allocatable.Runners.IntVal < filteredNodes[j].Status.Allocatable.Runners.IntVal
	})
	return filteredNodes
}

// findNodeSuitsForRunner returns node with capacity for runner
func (s *Scheduler) findNodeSuitsForRunner(runner *types.Runner, nodes []*types.Node) *types.Node {
	for _, node := range nodes {
		if node.Status.Condition.Status == types.Ready {
			cpu := node.Status.Allocatable.Cpu.IntVal
			memory := node.Status.Allocatable.Memory.IntVal
			runners := node.Status.Allocatable.Runners.IntVal

			for _, binding := range node.Status.Binding {
				cpu -= binding.Cpu.IntVal
				memory -= binding.Memory.IntVal
				runners += 1
			}

			if runners > 0 && cpu >= runner.Config.Cpu.IntVal && memory >= runner.Config.Memory.IntVal {
				logger.Infof("Node %s has capacity for runners %s", node.ID, runner.ID)
				return node
			}
		}

	}
	return nil
}

func (s *Scheduler) assignRunner(runner *types.Runner, node *types.Node) error {
	// TODO: implement ectd lock here for runner and node
	// update node binding and save node to etcd
	node.Status.Binding = append(node.Status.Binding, types.ResourceBinding{
		Cpu:      runner.Config.Cpu,
		Memory:   runner.Config.Memory,
		RunnerID: runner.ID,
	})
	err := s.nodePoolManager.SaveNode(*node)
	if err != nil {
		logger.Errorf("Assign Runner failed to save node %s, error: %v", node.ID, err)
		return err
	}

	// update runner status to create and save runner to etcd
	runner.NodeID = node.ID
	runner.Condition.Status = types.Creating
	err = s.runnerManager.SaveRunner(runner)
	if err != nil {
		logger.Errorf("Assign Runner failed to save runner %s, error: %v", runner.ID, err)
	}
	return nil
}
