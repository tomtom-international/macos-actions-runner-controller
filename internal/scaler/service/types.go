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
