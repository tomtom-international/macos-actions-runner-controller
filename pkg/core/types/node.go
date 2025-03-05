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
	Name string    `json:"name"`
	ID   utils.UID `json:"id"`

	Status                Status    `json:"status"`
	RegistrationTimestamp time.Time `json:"registrationTimestamp"`
	NodeInfo              NodeInfo  `json:"nodeInfo,omitempty"`
}

type Status struct {
	Capacity    Resources         `json:"capacity"`
	Allocatable Resources         `json:"allocatable"`
	Binding     []ResourceBinding `json:"binding,omitempty"`
	Condition   Condition         `json:"condition"`
}

type Resources struct {
	Cpu     utils.Int32String `json:"cpu"`
	Memory  utils.Int32String `json:"memory"`
	Runners utils.Int32String `json:"runners"`
}

type ResourceBinding struct {
	Cpu      utils.Int32String `json:"cpu"`
	Memory   utils.Int32String `json:"memory"`
	RunnerID utils.UID         `json:"runnerId"`
}

type Condition struct {
	Status             NodeStatus `json:"status"`
	Healthy            bool       `json:"healthy"`
	LastHeartbeatTime  time.Time  `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime time.Time  `json:"lastTransitionTime,omitempty"`
	Message            string     `json:"message,omitempty"`
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
	Node               Node                   `json:"node,omitempty"`
	Message            string                 `json:"message"`
}

type NodeHeartbeatRequest struct {
	NodeName    string     `json:"nodeName"`
	Allocatable Resources  `json:"allocatable"`
	Status      NodeStatus `json:"status"`
	Message     string     `json:"message,omitempty"`
}

type WatcherNodesUpdate struct {
	UpdatedNodes []Node `json:"updatedNodes"`
	DeletedNodes []Node `json:"deletedNodes"`
}
