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
	"time"

	t "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type RunnerStatus string

const (
	// Running when runner is running on the node
	Running RunnerStatus = "running"
	// Pending when controller received runner request and runner waiting
	// for a node to be assigned
	Pending RunnerStatus = "pending"
	// Creating when runner is assigned to the node and waiting for the
	// Tarter to notify that Runner is running
	Creating RunnerStatus = "creating"
	// Failed when runner has failed to start or failed while executing a job
	Failed RunnerStatus = "failed"
	// Finished when runner is finished
	Finished RunnerStatus = "finished"
)

type Runner struct {
	Condition     RunnerCondition `json:"condition"`
	ID            utils.UID       `json:"id"`
	NodeID        utils.UID       `json:"nodeId,omitempty"`
	GhaRunnerName string          `json:"ghaRunnerName,omitempty"`
	Config        RunnerConfig    `json:"config"`
}

type RunnerCondition struct {
	Status             RunnerStatus `json:"status"`
	Message            string       `json:"message,omitempty"`
	CreationTimestamp  time.Time    `json:"lastHeartbeatTime"`
	LastTransitionTime time.Time    `json:"lastTransitionTime,omitempty"`
	CreateRequestID    string       `json:"createRequestId,omitempty"`
}

type RunnerConfig struct {
	StartupProbe   *t.Probe `json:"startupProbe,omitempty" yaml:"startupProbe,omitempty"`
	LivenessProbe  *t.Probe `json:"livenessProbe,omitempty" yaml:"livenessProbe,omitempty"`
	ReleaseVersion string   `json:"releaseVersion" yaml:"releaseVersion"`
	RunnerGroup    string   `json:"runnerGroup" yaml:"runnerGroup"`
	Name           string   `json:"name" yaml:"name"`
	BaseImage      string   `json:"baseImage" yaml:"baseImage"`
	// JitConfig is ACTIONS_RUNNER_INPUT_JITCONFIG environment variable passed by Actions Runner Controller.
	// Temporary solution before Runners Listener is implemented.
	JitConfig              string            `json:"jitConfig,omitempty" yaml:"jitConfig,omitempty"`
	CacheVolumePath        string            `json:"cacheVolumePath" yaml:"cacheVolumePath"`
	RunnerHostname         string            `json:"hostname" yaml:"hostname"`
	RunnerHosts            []RunnerHosts     `json:"hosts" yaml:"hosts"`
	RunnerLabels           []string          `json:"labels" yaml:"labels"`
	Memory                 utils.Int32String `json:"memory" yaml:"memory"`
	CPU                    utils.Int32String `json:"cpu" yaml:"cpu"`
	SoftnetNetwork         SoftnetNetwork    `json:"softnetNetwork" yaml:"softnetNetwork"`
	NoGraphics             bool              `json:"noGraphics" yaml:"noGraphics"`
	DisableRootDiskOptions bool              `json:"disableRootDiskOptions" yaml:"disableRootDiskOptions"`
	RestartOnFailure       bool              `json:"restartOnFailure" yaml:"restartOnFailure"`
}

type RunnerHosts struct {
	IP        string   `yaml:"ip"`
	Hostnames []string `yaml:"hostnames"`
}

type SoftnetNetwork struct {
	AllowedCIDRs []string `yaml:"allowCIDRs"`
	Enable       bool     `yaml:"enable"`
}

type WatcherRunnersUpdate struct {
	UpdatedRunners []Runner `json:"updatedRunners"`
	DeletedRunners []Runner `json:"deletedRunners"`
}

type RunnerStatusUpdate struct {
	ID            utils.UID    `json:"id"`
	Status        RunnerStatus `json:"status"`
	GhaRunnerName string       `json:"ghaRunnerName,omitempty"`
	Message       string       `json:"message,omitempty"`
}
