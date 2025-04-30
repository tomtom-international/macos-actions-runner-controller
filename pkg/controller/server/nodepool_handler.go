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
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"net/http"
)

func GetNodeListHandler(c ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			return
		}

		var resp []byte
		var err error

		nodes, err := c.GetNodeList()

		if err == nil {
			resp, err = json.Marshal(nodes)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to list nodes: %v", err.Error()),
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		_, err = w.Write(resp)
		if err != nil {
			return
		}
	}
}

func RegisterNodeHandler(c ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var nodeRegistration types.NodeRegistrationRequest
		err := json.NewDecoder(r.Body).Decode(&nodeRegistration)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := types.NodeRegistrationResponse{
				RegistrationStatus: types.RegistrationError,
				Message:            "Failed to read registration request",
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		status, node, err := c.RegisterNewNode(nodeRegistration)
		if err != nil {
			if node == nil {
				w.WriteHeader(http.StatusInternalServerError)
				response := types.NodeRegistrationResponse{
					RegistrationStatus: status,
					Message:            fmt.Sprintf("Node is nil"),
				}
				err = json.NewEncoder(w).Encode(response)
				if err != nil {
					return
				}
				return
			} else {
				w.WriteHeader(http.StatusConflict)
				response := types.NodeRegistrationResponse{
					RegistrationStatus: status,
					Node:               *node,
					Message:            fmt.Sprintf("Node %s already registered or deregistered for scheduling", node.Name),
				}
				err := json.NewEncoder(w).Encode(response)
				if err != nil {
					return
				}
				return
			}
		}

		w.WriteHeader(http.StatusCreated)
		response := types.NodeRegistrationResponse{
			RegistrationStatus: status,
			Node:               *node,
			Message:            fmt.Sprintf("Node %s registered with id %s", node.Name, node.ID),
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}
}

func GetNodeHandler(c ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		vars := mux.Vars(r)
		nodeID := utils.UID(vars["node_uid"])
		if nodeID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Node id is required",
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}

		var resp []byte
		var err error

		node, err := c.GetNodeInfo(nodeID)

		if err == nil {
			if node == nil {
				w.WriteHeader(http.StatusNotFound)
				response := core.SimpleResponse{
					Status:  http.StatusNotFound,
					Message: fmt.Sprintf("Node %s not found", nodeID),
				}
				err = json.NewEncoder(w).Encode(response)
				if err != nil {
					return
				}
				return
			}
			resp, err = json.Marshal(node)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to get node: %v", err.Error()),
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		_, err = w.Write(resp)
		if err != nil {
			return
		}
	}
}

func NodeHeartbeatHandler(c ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		vars := mux.Vars(r)
		nodeID := utils.UID(vars["node_uid"])
		if nodeID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Node id is required",
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}

		var body types.NodeHeartbeatRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Failed to read node heartbeat request",
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		status, node, err := c.ProcessNodeHeartBeat(nodeID, body)
		if err != nil {
			if status == types.HeartbeatDeregisteredNode {
				w.WriteHeader(http.StatusNotAcceptable)
				response := core.SimpleResponse{
					Status:  http.StatusNotAcceptable,
					Message: fmt.Sprintf("Node %s disabled for scheduling", nodeID),
				}
				err = json.NewEncoder(w).Encode(response)
				if err != nil {
					return
				}
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to process node heartBeat: %v", err.Error()),
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		if status == types.HeartbeatNodeNotFound {
			w.WriteHeader(http.StatusNotFound)
			response := core.SimpleResponse{
				Status:  http.StatusNotFound,
				Message: fmt.Sprintf("Node %s not found", nodeID),
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}

		err = json.NewEncoder(w).Encode(node)
		if err != nil {
			return
		}
	}
}

func UpdateNodeStatusHandler(c ControllerInterface, nodeStatus types.NodeStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		vars := mux.Vars(r)
		nodeID := utils.UID(vars["node_uid"])
		if nodeID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Node id is required",
			}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}

		var err error
		var node *types.Node
		if nodeStatus == types.NotReady {
			node, err = c.ReEnable(nodeID)
		}
		if nodeStatus == types.Disabled {
			node, err = c.DisableNode(nodeID)
		}
		if nodeStatus == types.Deregistered {
			node, err = c.DeregisterNode(nodeID)
		}

		if node == nil && err == nil {
			w.WriteHeader(http.StatusNotFound)
			response := core.SimpleResponse{
				Status:  http.StatusNotFound,
				Message: fmt.Sprintf("Node %s not found", nodeID),
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				return
			}
			return
		}
		response := core.SimpleResponse{
			Status:  http.StatusOK,
			Message: fmt.Sprintf("Node %s disabled", nodeID),
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}
}
