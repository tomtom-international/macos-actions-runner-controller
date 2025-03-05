/*
 * Copyright 2025 TomTom N.V.
 * Copyright 2015 The Kubernetes Authors.
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
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"net"
	"net/http"
	"os"
	"time"
)

// ControllerInterface contains all the controller methods required for the server.
type ControllerInterface interface {
	RegisterNewNode(node types.NodeRegistrationRequest) (types.NodeRegistrationStatus, *types.Node, error)
	ProcessNodeHeartBeat(nodeID utils.UID, heartbeat types.NodeHeartbeatRequest) (types.NodeHeartbeatStatus, *types.Node, error)
	DeregisterNode(nodeID utils.UID) (*types.Node, error)
	GetNodeInfo(nodeID utils.UID) (*types.Node, error)
	GetNodeList() ([]types.Node, error)
	DisableNode(nodeID utils.UID) (*types.Node, error)
	ReEnable(nodeID utils.UID) (*types.Node, error)
	RemoveNodesWatcherHandler(conn *websocket.Conn)
	AddNodesWatcher(conn *websocket.Conn, notificationChan chan types.WatcherNodesUpdate)
	GetRunnerInfo(runnerID utils.UID) (*types.Runner, error)
	GetRunnerList() ([]types.Runner, error)
	AddRunnersWatcher(conn *websocket.Conn, notificationChan chan types.WatcherRunnersUpdate)
	RemoveRunnersWatcherHandler(conn *websocket.Conn)
	ProcessRunnersStatusUpdate(update types.RunnerStatusUpdate) (*types.Runner, error)
}

func NewServer(controller ControllerInterface) http.Handler {
	r := mux.NewRouter()
	core.SetupGenericHandlers(r)
	s := r.PathPrefix("/api/v1").Subrouter()
	SetupAuditRoutes(s)
	SetupNodePoolRoutes(s, controller)
	SetupWatcherRoutes(r, controller)
	SetupRunnersRoutes(s, controller)
	// TODO refactor cors middleware
	r.Use(mux.CORSMethodMiddleware(r))
	return r
}

func ListenAndServeControllerServer(controller ControllerInterface, address net.IP, port string) {
	logger.Infof("Starting to listen address %s port %s", address, port)
	handler := NewServer(controller)

	s := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", address, port),
		Handler: handler,
		// To avoid Slowloris attacks and control websocket connections.
		ReadTimeout:    2 * 60 * time.Minute,
		WriteTimeout:   1 * 60 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		logger.Errorf("Failed to listen and serve %v", err)
		os.Exit(1)
	}
}
