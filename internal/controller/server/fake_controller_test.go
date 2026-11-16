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

package server

import (
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/mock"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type FakeController struct {
	mock.Mock
}

func nodeAt(args mock.Arguments, i int) *types.Node {
	n, _ := args.Get(i).(*types.Node)
	return n
}

func runnerAt(args mock.Arguments, i int) *types.Runner {
	r, _ := args.Get(i).(*types.Runner)
	return r
}

func (f *FakeController) RegisterNewNode(node types.NodeRegistrationRequest) (types.NodeRegistrationStatus, *types.Node, error) {
	args := f.Called(node)
	return args.Get(0).(types.NodeRegistrationStatus), nodeAt(args, 1), args.Error(2)
}

func (f *FakeController) ProcessNodeHeartBeat(
	nodeID utils.UID, heartbeat types.NodeHeartbeatRequest,
) (types.NodeHeartbeatStatus, *types.Node, error) {

	args := f.Called(nodeID, heartbeat)
	return args.Get(0).(types.NodeHeartbeatStatus), nodeAt(args, 1), args.Error(2)
}

func (f *FakeController) DeregisterNode(nodeID utils.UID) (*types.Node, error) {
	args := f.Called(nodeID)
	return nodeAt(args, 0), args.Error(1)
}

func (f *FakeController) GetNodeInfo(nodeID utils.UID) (*types.Node, error) {
	args := f.Called(nodeID)
	return nodeAt(args, 0), args.Error(1)
}

func (f *FakeController) GetNodeList() ([]types.Node, error) {
	args := f.Called()
	nodes, _ := args.Get(0).([]types.Node)
	return nodes, args.Error(1)
}

func (f *FakeController) DisableNode(nodeID utils.UID) (*types.Node, error) {
	args := f.Called(nodeID)
	return nodeAt(args, 0), args.Error(1)
}

func (f *FakeController) ReEnable(nodeID utils.UID) (*types.Node, error) {
	args := f.Called(nodeID)
	return nodeAt(args, 0), args.Error(1)
}

func (f *FakeController) RemoveNodesWatcherHandler(conn *websocket.Conn) {
	f.Called(conn)
}

func (f *FakeController) AddNodesWatcher(conn *websocket.Conn, notificationChan chan types.WatcherNodesUpdate) {
	f.Called(conn, notificationChan)
}

func (f *FakeController) GetRunnerInfo(runnerID utils.UID) (*types.Runner, error) {
	args := f.Called(runnerID)
	return runnerAt(args, 0), args.Error(1)
}

func (f *FakeController) GetRunnerList() ([]types.Runner, error) {
	args := f.Called()
	runners, _ := args.Get(0).([]types.Runner)
	return runners, args.Error(1)
}

func (f *FakeController) AddRunnersWatcher(conn *websocket.Conn, notificationChan chan types.WatcherRunnersUpdate) {
	f.Called(conn, notificationChan)
}

func (f *FakeController) RemoveRunnersWatcherHandler(conn *websocket.Conn) {
	f.Called(conn)
}

func (f *FakeController) ProcessRunnersStatusUpdate(update types.RunnerStatusUpdate) (*types.Runner, error) {
	args := f.Called(update)
	return runnerAt(args, 0), args.Error(1)
}
