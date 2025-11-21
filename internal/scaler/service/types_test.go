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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

func TestTerminationState_String(t *testing.T) {
	tests := []struct {
		state    TerminationState
		expected string
	}{
		{TerminationStateSelected, "selected"},
		{TerminationStateDraining, "draining"},
		{TerminationStateReadyToTerminate, "ready_to_terminate"},
		{TerminationStateTerminating, "terminating"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.state))
		})
	}
}

func TestNode_JSONMarshalling(t *testing.T) {
	node := Node{
		ID:               utils.UID("node-123"),
		InstanceID:       "i-1234567890",
		Hostname:         "ip-10-0-1-100.ec2.internal",
		Status:           types.Ready,
		ActiveRunners:    2,
		TerminationState: TerminationStateDraining,
	}

	// Marshal to JSON
	data, err := json.Marshal(node)
	require.NoError(t, err)

	// Unmarshal back
	var unmarshalled Node
	err = json.Unmarshal(data, &unmarshalled)
	require.NoError(t, err)

	assert.Equal(t, node.ID, unmarshalled.ID)
	assert.Equal(t, node.InstanceID, unmarshalled.InstanceID)
	assert.Equal(t, node.Hostname, unmarshalled.Hostname)
	assert.Equal(t, node.Status, unmarshalled.Status)
	assert.Equal(t, node.ActiveRunners, unmarshalled.ActiveRunners)
	assert.Equal(t, node.TerminationState, unmarshalled.TerminationState)
}

func TestScalingOperation_JSONMarshalling(t *testing.T) {
	op := ScalingOperation{
		GroupName:    "my-asg",
		OriginalSize: 10,
		TargetSize:   8,
		NodesToTerminate: []Node{
			{
				ID:               utils.UID("node-1"),
				InstanceID:       "i-111",
				Hostname:         "host-1",
				Status:           types.Disabled,
				ActiveRunners:    0,
				TerminationState: TerminationStateReadyToTerminate,
			},
			{
				ID:               utils.UID("node-2"),
				InstanceID:       "i-222",
				Hostname:         "host-2",
				Status:           types.Ready,
				ActiveRunners:    1,
				TerminationState: TerminationStateDraining,
			},
		},
		Attempts: 1,
	}

	// Marshal to JSON
	data, err := json.Marshal(op)
	require.NoError(t, err)

	// Unmarshal back
	var unmarshalled ScalingOperation
	err = json.Unmarshal(data, &unmarshalled)
	require.NoError(t, err)

	assert.Equal(t, op.GroupName, unmarshalled.GroupName)
	assert.Equal(t, op.OriginalSize, unmarshalled.OriginalSize)
	assert.Equal(t, op.TargetSize, unmarshalled.TargetSize)
	assert.Equal(t, op.Attempts, unmarshalled.Attempts)
	assert.Len(t, unmarshalled.NodesToTerminate, 2)
	assert.Equal(t, op.NodesToTerminate[0].ID, unmarshalled.NodesToTerminate[0].ID)
	assert.Equal(t, op.NodesToTerminate[1].ID, unmarshalled.NodesToTerminate[1].ID)
}
