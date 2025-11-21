package service

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type TerminationState string

const (
	TerminationStateSelected         TerminationState = "selected"
	TerminationStateDraining         TerminationState = "draining"
	TerminationStateReadyToTerminate TerminationState = "ready_to_terminate"
	TerminationStateTerminating      TerminationState = "terminating"
)

// Node represents a node in a node group.
type Node struct {
	ID               utils.UID        `json:"id"`
	InstanceID       string           `json:"instance_id"`
	Hostname         string           `json:"hostname"`
	Status           types.NodeStatus `json:"status"`
	TerminationState TerminationState `json:"termination_state"`
	ActiveRunners    int              `json:"active_runners"`
}

// ScalingOperation represents a scaling operation for a node group.
type ScalingOperation struct {
	GroupName        string `json:"group_name"`
	NodesToTerminate []Node `json:"nodes_to_terminate"`
	OriginalSize     int    `json:"original_size"`
	TargetSize       int    `json:"target_size"`
	Attempts         int    `json:"attempts"`
}
