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

package types

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"time"
)

type NodeStatus string

const (
	Ready        NodeStatus = "ready"
	NotReady     NodeStatus = "not-ready"
	Disabled     NodeStatus = "disabled"
	Deregistered NodeStatus = "deregistered"
	Unknown      NodeStatus = "unknown"
)

type NodeRegistrationStatus string

const (
	Registered        NodeRegistrationStatus = "registered"
	RegistrationError NodeRegistrationStatus = "registration-error"
	NodeDisabled      NodeRegistrationStatus = "node-disabled"
	NodeDeregistered  NodeRegistrationStatus = "node-deregistered"
	NodeAlreadyExists NodeRegistrationStatus = "node-already-exists"
)

type NodeHeartbeatStatus string

const (
	HeartbeatSuccess          NodeHeartbeatStatus = "success"
	HeartbeatFailed           NodeHeartbeatStatus = "failed"
	HeartbeatDeregisteredNode NodeHeartbeatStatus = "deregistered-node"
	HeartbeatNodeNotFound     NodeHeartbeatStatus = "node-not-found"
)

type Node struct {
	RegistrationTimestamp time.Time `json:"registrationTimestamp"`
	NodeInfo              NodeInfo  `json:"nodeInfo,omitempty"`
	Name                  string    `json:"name"`
	ID                    utils.UID `json:"id"`
	Status                Status    `json:"status"`
}

type Status struct {
	Condition   Condition         `json:"condition"`
	Binding     []ResourceBinding `json:"binding,omitempty"`
	Capacity    Resources         `json:"capacity"`
	Allocatable Resources         `json:"allocatable"`
}

type Resources struct {
	Cpu     utils.Int32String `json:"cpu"`
	Memory  utils.Int32String `json:"memory"`
	Runners utils.Int32String `json:"runners"`
}

type ResourceBinding struct {
	RunnerID utils.UID         `json:"runnerId"`
	Cpu      utils.Int32String `json:"cpu"`
	Memory   utils.Int32String `json:"memory"`
}

type Condition struct {
	LastHeartbeatTime  time.Time  `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime time.Time  `json:"lastTransitionTime,omitempty"`
	Status             NodeStatus `json:"status"`
	Message            string     `json:"message,omitempty"`
	Healthy            bool       `json:"healthy"`
}

type NodeInfo struct {
	TarterVersion string  `json:"tarterVersion,omitempty"`
	TartVersion   string  `json:"tartVersion,omitempty"`
	HostOSVersion string  `json:"hostOSVersion,omitempty"`
	Address       Address `json:"address,omitempty"`
}

type Address struct {
	IP       string `json:"ip,omitempty"`
	Port     string `json:"port,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

type NodeRegistrationRequest struct {
	NodeName string    `json:"nodeName"`
	NodeInfo NodeInfo  `json:"nodeInfo,omitempty"`
	Capacity Resources `json:"capacity"`
}

type NodeRegistrationResponse struct {
	RegistrationStatus NodeRegistrationStatus `json:"registrationStatus"`
	Message            string                 `json:"message"`
	Node               Node                   `json:"node,omitempty"`
}

type NodeHeartbeatRequest struct {
	NodeName    string     `json:"nodeName"`
	Status      NodeStatus `json:"status"`
	Message     string     `json:"message,omitempty"`
	Allocatable Resources  `json:"allocatable"`
}

type WatcherNodesUpdate struct {
	UpdatedNodes []Node `json:"updatedNodes"`
	DeletedNodes []Node `json:"deletedNodes"`
}
