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
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/sqs"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/config"
	np "github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/nodepool"
	r "github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/runners"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/server"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type Controller struct {
	EtcdClient             *etcd.EtcdClient
	nodePoolManager        *np.Manager
	sqsRunnerRequestClient *sqs.SQSClient
	runnerManager          *r.Manager
	runnerUpdateCn         chan types.Runner
	unhealthyNodesCh       chan *types.Node
}

func NewController(configuration config.ControllerConfig) (*Controller, error) {
	etcdClient, err := etcd.NewEtcdClient(
		configuration.EtcdEndpoints, func(err error) {
			logger.Errorf("Etcd Error: %s", err.Error())
		})
	if err != nil {
		return nil, err
	}
	sqsClient, err := sqs.NewClient(
		&sqs.SQSConfig{
			QueueURL:  configuration.AwsRunnerRequestSQSUrl,
			AWSRegion: configuration.AwsRegion,
			ErrHandle: func(err error) {
				logger.Errorf("Sqs error: %s", err.Error())
			},
		},
	)
	if err != nil {
		return nil, err
	}
	UnhealthyNodesCh := make(chan *types.Node, 100)
	return &Controller{
		EtcdClient:             etcdClient,
		sqsRunnerRequestClient: sqsClient,
		runnerUpdateCn:         make(chan types.Runner, 100),
		unhealthyNodesCh:       UnhealthyNodesCh,
		nodePoolManager:        np.NewManager(etcdClient, config.EtcdNodePoolKey, configuration.EtcdNodeDeregisterLease, UnhealthyNodesCh),
		runnerManager:          r.NewManager(etcdClient, config.EtcdRunnersKey, configuration.EtcdRunnerFinishedLease),
	}, nil
}

func (c *Controller) ListenAndServe(configuration config.ControllerConfig) {
	address := net.ParseIP(configuration.Address)
	port := configuration.Port
	server.ListenAndServeControllerServer(c, address, port)
}

func (c *Controller) Run(ctx context.Context, wg *sync.WaitGroup) {
	// TODO: At launch, read all nodes from etcd and start node heartbeat watchers
	// TODO: At launch, read all runners from etcd and start runner create watchers
	c.checkRegisteredNodes()

	scheduler := newScheduler(c.nodePoolManager, c.runnerManager)
	wg.Add(3)
	go func() {
		defer wg.Done()
		c.listenForNewRunners(ctx, c.readSQSMessages)
	}()
	go func() {
		defer wg.Done()
		scheduler.startScheduler(ctx)
	}()
	go func() {
		defer wg.Done()
		c.checkUnhealthyNodes(ctx)
	}()
}

func (c *Controller) RegisterNewNode(request types.NodeRegistrationRequest) (types.NodeRegistrationStatus, *types.Node, error) {
	existingNode, err := c.nodePoolManager.GetNodeByName(request.NodeName)

	// TODO: Validate node registration request. Check node capacity and node info.

	if err != nil {
		logger.Errorf("Failed to get node by name %s, error: %v", request.NodeName, err)
		return types.RegistrationError, existingNode, err
	}
	if existingNode != nil {
		if existingNode.Status.Condition.Status == types.Disabled {
			logger.Warnf("Received node registration request for disabled node %s.", request.NodeName)
			return types.NodeDisabled, existingNode, fmt.Errorf("node %s disabled", request.NodeName)
		}
		if existingNode.Status.Condition.Status == types.Deregistered {
			logger.Warnf("Received node registration request for deregistered node %s. Decline node registration", request.NodeName)
			return types.NodeDeregistered, existingNode, fmt.Errorf("node %s is deregistered. Waiting to be removed", request.NodeName)
		} else {
			logger.Warnf("Received node registration request for active node %s", request.NodeName)
			c.nodePoolManager.AddNodeHeartbeatWatcher(existingNode.ID)
			return types.NodeAlreadyExists, existingNode, fmt.Errorf("node %s already registered", request.NodeName)
		}
	}

	nodeID := utils.NewUUID()
	newNode := types.Node{
		Name: request.NodeName,
		ID:   nodeID,
		Status: types.Status{
			Capacity: request.Capacity,
			Condition: types.Condition{
				Status:  types.NotReady,
				Healthy: false,
				Message: "Node registration successful",
			},
		},
		RegistrationTimestamp: time.Now(),
		NodeInfo:              request.NodeInfo,
	}
	err = c.nodePoolManager.SaveNode(newNode)
	if err != nil {
		logger.Errorf("Failed to save node %s, error: %v", request.NodeName, err)
		return types.RegistrationError, nil, err
	}
	logger.Infof("Node %s with id %s registered successfully", request.NodeName, nodeID)
	c.nodePoolManager.AddNodeHeartbeatWatcher(nodeID)
	return types.Registered, &newNode, nil
}

func (c *Controller) ProcessNodeHeartBeat(
	nodeID utils.UID,
	heartbeat types.NodeHeartbeatRequest,
) (types.NodeHeartbeatStatus, *types.Node, error) {

	// check by Node ID if node exists in etcd
	registeredNode, err := c.nodePoolManager.GetNodeByID(nodeID)
	if err != nil {
		logger.Errorf("Failed to get node with id %s, error: %v", nodeID, err)
		return types.HeartbeatFailed, nil, err
	}
	if registeredNode == nil {
		logger.Errorf("Received heartbeat for non-existing Node %s", nodeID)
		return types.HeartbeatNodeNotFound, nil, nil
	}

	if !c.nodePoolManager.IsHeartbeatWatching(nodeID) {
		c.nodePoolManager.AddNodeHeartbeatWatcher(registeredNode.ID)
	}

	// TODO: Validations
	// Validate allocatable resources < capacity resources
	// If node not found -> return error to register node
	// if name != registeredNode.Name  return error ?? maybe re register node with new name if it active or not

	if registeredNode.Status.Condition.Status == types.Deregistered {
		return types.HeartbeatDeregisteredNode, nil, fmt.Errorf("node %s deregistered", registeredNode.Name)
	}

	if registeredNode.Status.Condition.Status != types.Disabled {
		registeredNode.Status.Condition.LastTransitionTime = time.Now()
		if heartbeat.Status != types.Ready {
			registeredNode.Status.Condition.Status = types.NotReady
			registeredNode.Status.Condition.Message = heartbeat.Message
		} else {
			registeredNode.Status.Condition.Status = types.Ready
			registeredNode.Status.Condition.Message = "Node is ready"
		}
	}

	registeredNode.Status.Allocatable = heartbeat.Allocatable
	registeredNode.Status.Condition.LastHeartbeatTime = time.Now()
	registeredNode.Status.Condition.Healthy = true

	err = c.nodePoolManager.SaveNode(*registeredNode)
	if err != nil {
		logger.Errorf("Failed to save node %s, error: %v", registeredNode.Name, err)
		return types.HeartbeatFailed, nil, err
	}
	return types.HeartbeatSuccess, registeredNode, nil
}

func (c *Controller) DeregisterNode(nodeID utils.UID) (*types.Node, error) {
	node, err := c.nodePoolManager.GetNodeByID(nodeID)
	if err != nil {
		logger.Errorf("Failed to get node %s, error: %v", nodeID, err)
		return nil, err
	}

	if node != nil {
		if node.Status.Condition.Status == types.Ready {
			return node, fmt.Errorf("node %s is active", node.Name)
		} else {
			node.Status.Condition.Status = types.Deregistered
			node.Status.Condition.Message = "Node deregistered"
			node.Status.Condition.LastTransitionTime = time.Now()
			err = c.nodePoolManager.SaveNodeWithLease(*node)
			if err != nil {
				logger.Errorf("Failed to deregister node %s, error: %v", node.Name, err)
				return nil, err
			}
		}
	}
	return node, nil
}

func (c *Controller) GetNodeInfo(nodeID utils.UID) (*types.Node, error) {
	node, err := c.nodePoolManager.GetNodeByID(nodeID)
	if err != nil {
		logger.Errorf("Failed to get node %s, error: %v", nodeID, err)
		return nil, err
	}
	return node, nil
}

func (c *Controller) GetNodeList() ([]types.Node, error) {
	nodeList, err := c.nodePoolManager.GetNodePool()
	if err != nil {
		logger.Errorf("Failed to get node list, error: %v", err)
		return nil, err
	}
	return nodeList, nil
}

func (c *Controller) DisableNode(nodeID utils.UID) (*types.Node, error) {
	node, err := c.nodePoolManager.GetNodeByID(nodeID)
	if err != nil {
		logger.Errorf("Failed to get node %s, error: %v", nodeID, err)
		return nil, err
	}
	if node != nil {
		if node.Status.Condition.Status == types.Deregistered || node.Status.Condition.Status == types.Disabled {
			return node, fmt.Errorf("node %s already in %s status", node.Name, node.Status.Condition.Status)
		}
		node.Status.Condition.Status = types.Disabled
		node.Status.Condition.Message = "Node disabled for placing new runners"
		node.Status.Condition.LastTransitionTime = time.Now()
		err = c.nodePoolManager.SaveNode(*node)
		if err != nil {
			logger.Errorf("Failed to disable node %s, error: %v", node.Name, err)
			return nil, err
		}
	}
	return node, nil
}

func (c *Controller) ReEnable(nodeID utils.UID) (*types.Node, error) {
	node, err := c.nodePoolManager.GetNodeByID(nodeID)
	if err != nil {
		logger.Errorf("Failed to get node %s, error: %v", nodeID, err)
		return nil, err
	}
	if node != nil {
		if node.Status.Condition.Status != types.Disabled {
			return node, fmt.Errorf("node %s in %s status. Enabling only disabled nodes", node.Name, node.Status.Condition.Status)
		}
		node.Status.Condition.Status = types.NotReady
		node.Status.Condition.Message = "Node re-enabled"
		node.Status.Condition.LastTransitionTime = time.Now()
		err = c.nodePoolManager.SaveNode(*node)
		if err != nil {
			logger.Errorf("Failed to enable node %s, error: %v", node.Name, err)
			return nil, err
		}
	}
	return node, nil
}

func (c *Controller) RemoveNodesWatcherHandler(conn *websocket.Conn) {
	c.nodePoolManager.RemoveNodesWatcherHandler(conn)
}

func (c *Controller) AddNodesWatcher(conn *websocket.Conn, notificationChan chan types.WatcherNodesUpdate) {
	c.nodePoolManager.AddNodesWatcher(conn, notificationChan)
}

func (c *Controller) UpdateDefaultRunnerConfig(runnerConfig *types.RunnerConfig) error {
	// TODO: implement config update
	// 1) validate runner config
	// 2) save runner config to etcd
	return nil
}

func (c *Controller) createRunner(runner *types.Runner) error {
	logger.Debugf("Creating runner with labels %+v", runner.Config.RunnerLabels)
	// Check if create runner request was already handled and runner is already created
	// to avoid duplicate runners
	if runner.Condition.CreateRequestID == "" {
		return fmt.Errorf("create request id is empty")
	}
	dupRunner, _ := c.runnerManager.GetRunnerByCreateRequestID(runner.Condition.CreateRequestID)
	if dupRunner != nil {
		return fmt.Errorf("runner with request id %s already exists", runner.Condition.CreateRequestID)
	}

	// generate runner fields
	runner.ID = utils.NewUUID()
	runner.Condition.Status = types.Pending
	runner.Condition.CreationTimestamp = time.Now()
	logger.Debugf("Runner assigned id %s", runner.ID)

	// TODO: merge runner config with default config

	// save runner to etcd
	err := c.runnerManager.SaveRunner(runner)
	if err != nil {
		logger.Errorf("Failed to create runner with id %s, error: %v", runner.ID, err)
		return fmt.Errorf("failed to create runner with label %s", runner.ID)
	}
	logger.Debugf("Runner %s created", runner.ID)
	return nil
}

func (c *Controller) GetRunnerInfo(runnerID utils.UID) (*types.Runner, error) {
	runner, err := c.runnerManager.GetRunnerByID(runnerID)
	if err != nil {
		logger.Errorf("Failed to get runner %s, error: %v", runnerID, err)
		return nil, err
	}
	return runner, nil
}

func (c *Controller) GetRunnerList() ([]types.Runner, error) {
	runners, err := c.runnerManager.GetRunners()
	if err != nil {
		logger.Errorf("Failed to get runners list, error: %v", err)
		return nil, err
	}
	return runners, nil
}

func (c *Controller) listenForNewRunners(ctx context.Context, handlerFunc func(message *sqsTypes.Message) error) {
	logger.Debugf("Listening for new messages from SQS queue")
	c.sqsRunnerRequestClient.PollForMessages(ctx, handlerFunc)
}

func (c *Controller) checkRegisteredNodes() {
	nodes, err := c.nodePoolManager.GetNodePool()
	if err != nil {
		logger.Errorf("Check registered nodes failed to get node pool, error: %v", err)
		return
	}

	for _, node := range nodes {
		if node.Status.Condition.Healthy && node.Status.Condition.LastHeartbeatTime.Add(1*time.Minute).Before(time.Now()) {
			node.Status.Condition.Healthy = false
			node.Status.Condition.Status = types.Unknown
			node.Status.Condition.Message = "Node heartbeat timeout exceeded on initial check"
			node.Status.Condition.LastTransitionTime = time.Now()

			err = c.nodePoolManager.SaveNode(node)
			if err != nil {
				logger.Errorf("Check registered nodes failed to save node %s, error: %v", node.Name, err)
				continue
			}
		}
	}
}

func (c *Controller) readSQSMessages(msg *sqsTypes.Message) error {
	runner := types.Runner{}
	logger.Debugf("Received message with id: %s from queue", *msg.MessageId)
	err := json.Unmarshal([]byte(*msg.Body), &runner)
	if err != nil {
		logger.Errorf("Failed to unmarshal message %s from queue: %s", *msg.MessageId, err.Error())
		return err
	}
	runner.Condition.CreateRequestID = *msg.MessageId

	err = c.createRunner(&runner)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) AddRunnersWatcher(conn *websocket.Conn, notificationChan chan types.WatcherRunnersUpdate) {
	c.runnerManager.AddRunnerWatcher(conn, notificationChan)
}

func (c *Controller) RemoveRunnersWatcherHandler(conn *websocket.Conn) {
	c.runnerManager.RemoveRunnersWatcherHandler(conn)
}

func (c *Controller) ProcessRunnersStatusUpdate(update types.RunnerStatusUpdate) (*types.Runner, error) {
	logger.Debugf("Processing runner status update %+v", update)
	runner, err := c.runnerManager.GetRunnerByID(update.ID)
	if err != nil {
		logger.Errorf("Failed to get runner %s, error: %v", update.ID, err)
		return nil, fmt.Errorf("failed to get runner %s", update.ID)
	}
	if runner == nil {
		logger.Errorf("Runner %s not found", update.ID)
		return nil, fmt.Errorf("runner %s not found", update.ID)
	}

	switch update.Status {
	case types.Running:
		logger.Debugf("Runner %s is running", runner.ID)
		runner.GhaRunnerName = update.GhaRunnerName
		runner.Condition.Status = update.Status
		// update node binding
		err = c.removeNodeBinding(runner.NodeID, runner.ID)
		if err != nil {
			return nil, err
		}
		err = c.runnerManager.SaveRunner(runner)
	case types.Failed:
		logger.Debugf("Runner %s failed. Error: %s", runner.ID, update.Message)
		runner.Condition.Status = update.Status
		runner.Condition.Message = update.Message
		// update node binding
		err = c.removeNodeBinding(runner.NodeID, runner.ID)
		if err != nil {
			return nil, err
		}
		err = c.runnerManager.SaveRunnerWithLease(runner)
	case types.Finished:
		logger.Debugf("Runner %s finished", runner.ID)
		runner.Condition.Status = update.Status
		err = c.runnerManager.SaveRunnerWithLease(runner)
	default:
		logger.Debugf("Runner %s is %s", runner.ID, runner.Condition.Status)
		runner.Condition.Status = update.Status
		err = c.runnerManager.SaveRunner(runner)
	}

	if err != nil {
		logger.Errorf("Failed to save runner %s, error: %v", runner.ID, err)
		return nil, fmt.Errorf("failed to save runner %s", runner.ID)
	}

	return runner, nil
}

func (c *Controller) removeNodeBinding(nodeID, runnerID utils.UID) error {
	logger.Debugf("Removing runner binding from node %s", nodeID)
	node, err := c.nodePoolManager.GetNodeByID(nodeID)
	if err != nil {
		logger.Errorf("Failed to get node %s runner assign to, error: %v", nodeID, err)
		return fmt.Errorf("failed to get node %s runner assign to", nodeID)
	}
	for i, binding := range node.Status.Binding {
		if binding.RunnerID == runnerID {
			node.Status.Binding = append(node.Status.Binding[:i], node.Status.Binding[i+1:]...)
			break
		}
	}
	err = c.nodePoolManager.SaveNode(*node)
	if err != nil {
		logger.Errorf("Failed to update node %s binding, error: %v", node.ID, err)
		return fmt.Errorf("failed to update node %s binding", node.ID)
	}
	return nil
}

func (c *Controller) checkUnhealthyNodes(ctx context.Context) {
	logger.Debugf("Starting orphan pod removal")

	for {
		select {
		case <-ctx.Done():
			logger.Warnf("Context cancelled, stopping orphan pod removal: %v", ctx.Err())
			return
		case node, ok := <-c.unhealthyNodesCh:
			if !ok {
				logger.Debugf("unhealthyNodesCh closed, exiting")
				return
			}

			for _, runner := range node.Status.Binding {
				logger.Infof("Found unhealthy runner %s on node %s", runner.RunnerID, node.ID)

				select {
				case <-ctx.Done():
					logger.Warnf("Context cancelled during runner processing: %v", ctx.Err())
					return
				default:
					_, err := c.ProcessRunnersStatusUpdate(types.RunnerStatusUpdate{
						ID:     runner.RunnerID,
						Status: types.Pending,
					})

					if err != nil {
						logger.Errorf("Failed to update status to Pending for runner %s: %v", runner.RunnerID, err)
						continue
					}
					logger.Infof("Updated unhealthy runner %s on node %s to Pending", runner.RunnerID, node.ID)
				}
			}
		}
	}
}
