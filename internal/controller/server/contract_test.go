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

package server_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/controller/server"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

var (
	contractNode = types.Node{
		RegistrationTimestamp: time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
		ID:                    "8a1c9e2e-0b7f-4e43-9d3e-2f6f1b6a0c11",
		Name:                  "mac-node-1",
		NodeInfo:              types.NodeInfo{TarterVersion: "1.2.3", Address: types.Address{IP: "10.0.0.5", Port: "8041"}},
		Status: types.Status{
			Condition:   types.Condition{Status: types.Ready, Healthy: true},
			Capacity:    types.Resources{CPU: utils.FromInt32(8), Memory: utils.FromInt32(16384), Runners: utils.FromInt32(2)},
			Allocatable: types.Resources{CPU: utils.FromInt32(4), Memory: utils.FromInt32(8192), Runners: utils.FromInt32(1)},
		},
	}
	contractRunner = types.Runner{
		ID:            "3f0e4b7a-6c1d-4f5a-8e2b-9d7c6b5a4e31",
		NodeID:        contractNode.ID,
		GhaRunnerName: "gha-runner-1",
		Condition:     types.RunnerCondition{Status: types.Running},
		Config:        types.RunnerConfig{Name: "runner-1", BaseImage: "macos:latest", RunnerLabels: []string{"macos"}},
	}
)

func newContractClient(t *testing.T) (*controller.Client, *server.FakeController) {
	t.Helper()
	f := &server.FakeController{}
	srv := httptest.NewServer(server.NewServer(f, "*"))
	t.Cleanup(srv.Close)
	c, err := controller.NewClient(controller.ClientConfig{Server: srv.URL, APIVersionPath: "/api/v1"})
	require.NoError(t, err)
	t.Cleanup(func() { f.AssertExpectations(t) })
	return c, f
}

func TestControllerClientContract(t *testing.T) {
	ctx := context.Background()

	t.Run("RegisterNode", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("RegisterNewNode", types.NodeRegistrationRequest{
			NodeName: contractNode.Name,
			NodeInfo: contractNode.NodeInfo,
			Capacity: contractNode.Status.Capacity,
		}).Return(types.Registered, &contractNode, nil)

		got, err := c.RegisterNode(ctx, contractNode)

		require.NoError(t, err)
		assert.Equal(t, contractNode, *got)
	})

	t.Run("RegisterNode already registered", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("RegisterNewNode", types.NodeRegistrationRequest{
			NodeName: contractNode.Name,
			NodeInfo: contractNode.NodeInfo,
			Capacity: contractNode.Status.Capacity,
		}).Return(types.NodeAlreadyExists, &contractNode, errors.New("exists"))

		got, err := c.RegisterNode(ctx, contractNode)

		var cErr *controller.ControllerErrors
		require.ErrorAs(t, err, &cErr)
		assert.Equal(t, string(types.NodeAlreadyExists), cErr.Code)
		assert.Equal(t, contractNode, *got)
	})

	t.Run("SendHeartbeat", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("ProcessNodeHeartBeat", contractNode.ID, types.NodeHeartbeatRequest{
			NodeName:    contractNode.Name,
			Allocatable: contractNode.Status.Allocatable,
			Status:      contractNode.Status.Condition.Status,
		}).Return(types.HeartbeatSuccess, &contractNode, nil)

		got, err := c.SendHeartbeat(ctx, contractNode)

		require.NoError(t, err)
		assert.Equal(t, contractNode, *got)
	})

	t.Run("GetNodeInfo", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("GetNodeInfo", contractNode.ID).Return(&contractNode, nil)

		got, err := c.GetNodeInfo(ctx, contractNode.ID)

		require.NoError(t, err)
		assert.Equal(t, contractNode, *got)
	})

	t.Run("GetNodes", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("GetNodeList").Return([]types.Node{contractNode}, nil)

		got, err := c.GetNodes(ctx)

		require.NoError(t, err)
		assert.Equal(t, []types.Node{contractNode}, got)
	})

	t.Run("DisableNode", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("DisableNode", contractNode.ID).Return(&contractNode, nil)

		require.NoError(t, c.DisableNode(ctx, contractNode.ID))
	})

	t.Run("EnableNode", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("ReEnable", contractNode.ID).Return(&contractNode, nil)

		require.NoError(t, c.EnableNode(ctx, contractNode.ID))
	})

	t.Run("DeregisterNode", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("DeregisterNode", contractNode.ID).Return(&contractNode, nil)

		require.NoError(t, c.DeregisterNode(ctx, contractNode.ID))
	})

	t.Run("GetRunnerInfo", func(t *testing.T) {
		c, f := newContractClient(t)
		f.On("GetRunnerInfo", contractRunner.ID).Return(&contractRunner, nil)

		got, err := c.GetRunnerInfo(ctx, contractRunner.ID)

		require.NoError(t, err)
		assert.Equal(t, contractRunner, *got)
	})

	t.Run("UpdateRunnerStatus", func(t *testing.T) {
		c, f := newContractClient(t)
		update := types.RunnerStatusUpdate{ID: contractRunner.ID, Status: types.Finished, GhaRunnerName: "gha-runner-1", Message: "done"}
		f.On("ProcessRunnersStatusUpdate", update).Return(&contractRunner, nil)

		require.NoError(t, c.UpdateRunnerStatus(ctx, update))
	})
}
