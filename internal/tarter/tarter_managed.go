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

package tarter

import (
	"context"
	"fmt"
	"time"

	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/events"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/node"
	tt "github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

func (t *Tarter) StartManagedTarter() {
	logger.Infof("Initializing Managed Tarter")
	if t.config == nil {
		logger.Fatalf("Managed configuration is not loaded")
	}
	controllerClient, err := controller.NewClient(t.config.GetControllerConfig())
	if err != nil {
		logger.Fatalf("Failed to create controller client: %s", err.Error())
	}
	t.controllerClient = controllerClient

	nodeInfo, err := t.getNodeInfo()
	if err != nil {
		logger.Fatalf("Failed to get node info: %s", err.Error())
	}
	nodeCapacity := types.Resources{
		CPU:     t.config.GetNodeCapacity().CPU,
		Memory:  t.config.GetNodeCapacity().Memory,
		Runners: t.config.GetNodeCapacity().MaxActiveRunners,
	}
	t.nodeManager = node.NewManager(
		t.StateManager,
		true,
		controllerClient,
		*nodeInfo,
		nodeCapacity,
		nodeStatusUpdateRetry,
	)

	logger.Infof("Start syncing node status with Controller")
	// Start go-routine to update the status.
	// It will report to the controller every heartbeatInterval and is aimed to provide regular status updates.
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.nodeManager.SyncNodeStatus()
			}
		}
	}()

	eventHandlers := map[string]func(event events.Event){
		tt.EventRunnerCreated:             t.handleRunnerCreated,
		tt.EventRunnerStopRequest:         t.handleRunnerStopping,
		tt.EventRunnerStatusUpdated:       t.handleManagedRunnerStatusUpdate,
		tt.EventRunnerLivenessHealthCheck: t.handleRunnerLivenessHealthCheck,
		tt.EventRunnerStartupHealthCheck:  t.handleRunnerStartupHealthCheck,
	}

	t.startEventHandling(eventHandlers)
}

func (t *Tarter) getNodeInfo() (*types.NodeInfo, error) {
	tartVersion, err := t.tartClient.GetTartVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get Tart version: %s", err.Error())
	}
	hostOSVersion, err := utils.GetMacOSVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get host OS version: %s", err.Error())
	}
	return &types.NodeInfo{
		TarterVersion: t.versionInfo.Version,
		TartVersion:   tartVersion,
		HostOSVersion: hostOSVersion,
		Address: types.Address{
			IP:       t.nodeIP,
			Port:     t.tarterServerPort,
			Hostname: t.nodeName,
		},
	}, nil
}

func (t *Tarter) handleManagedRunnerStatusUpdate(event events.Event) {
	r := event.Payload.(*tt.Runner)
	if state, exist := t.StateManager.GetRunnerState(r.ID); exist {
		logger.Debugf("Tarter handling runner <%s> status update event for runner id: %s",
			state.Status, r.ID)

		if state.Status == tt.Running {
			if r.Config.LivenessProbe != nil || r.Config.StartupProbe != nil {
				t.addProber(r)
			}
		}
		ctx := context.Background()
		var status types.RunnerStatus
		switch state.Status {
		case tt.Created:
			// We do not need to send the Created status to the controller
			return
		case tt.Stopped:
			status = types.Finished
		case tt.Stopping:
			// We do not need to send the Stopping status to the controller
			return
		case tt.Running:
			status = types.Running
		case tt.Failed:
			status = types.Failed
		case tt.Finished:
			status = types.Finished
		}
		statusUpdate := types.RunnerStatusUpdate{
			ID:            state.Runner.ID,
			Status:        status,
			Message:       state.ErrorMessage,
			GhaRunnerName: state.Runner.GhaRunnerName,
		}
		logger.Debugf("Updating runner status <%s> to controller for runner id: %s", state.Status, r.ID)
		t.nodeManager.SyncNodeStatusOnce()
		err := t.controllerClient.UpdateRunnerStatus(ctx, statusUpdate)
		if err != nil {
			logger.Errorf("Failed to update runner status: %v", err)
		}
	}
}
