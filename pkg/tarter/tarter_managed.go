package tarter

import (
	"context"
	"fmt"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/events"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/node"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"time"
)

func (t *Tarter) StartManagedTarter() {
	logger.Infof("Initializing Managed Tarter")
	if t.config == nil {
		logger.Fatalf("Managed configuration is not loaded")
	}
	controllerClient, err := controller.NewClient(t.config.GetControllerConfig())
	if err != nil {
		logger.Fatalf("Failed to create controller client: %v", err.Error())
	}
	t.controllerClient = controllerClient

	nodeInfo, err := t.getNodeInfo()
	if err != nil {
		logger.Fatalf("Failed to get node info: %v", err.Error())
	}
	nodeCapacity := types.Resources{
		Cpu:     t.config.GetNodeCapacity().Cpu,
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
		return nil, fmt.Errorf("failed to get Tart version: %v", err)
	}
	hostOSVersion, err := utils.GetMacOSVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get host OS version: %v", err)
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
