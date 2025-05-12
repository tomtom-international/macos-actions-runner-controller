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

package config

import (
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	u "github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

const (
	StandaloneMode TarterMode = "standalone"
	ManagedMode    TarterMode = "managed"
)

type TarterMode string

type TarterConfig struct {
	TartPath            string `env:"TART_PATH" envDefault:"/opt/homebrew/bin/tart"`
	GhAppOrg            string `env:"GH_APP_ORG"`
	GhAppPrivateKeyFile string `env:"GH_APP_PRIVATE_KEY_FILE" envDefault:"/opt/tarter/config/.gh_app_private_key.pem"`
	GhAppPrivateKey     string `env:"GH_APP_PRIVATE_KEY"`
	Port                string `env:"PORT" envDefault:"8041"`
	Mode                string `env:"TARTER_MODE"`
	ConfigFolder        string `env:"TARTER_CONFIG_PATH" envDefault:"/opt/tarter/config"`
	GhAppInstallationID int    `env:"GH_APP_INSTALLATION_ID"`
	GhAppID             int    `env:"GH_APP_ID"`
	LogDebug            bool   `env:"LOG_DEBUG" envDefault:"false"`
	LogJSON             bool   `env:"LOG_JSON" envDefault:"false"`
	LogStacktrace       bool   `env:"LOG_STACKTRACE" envDefault:"false"`
	LogCaller           bool   `env:"LOG_CALLER" envDefault:"false"`
}

type Config interface {
	GetRunnersConfig() []types.RunnerConfig
	GetControllerConfig() ControllerConfig
	GetNodeCapacity() NodeCapacity
	ReadConfig(path string) error
}

type ControllerConfig struct {
	Server         string `json:"server" yaml:"server"`
	APIVersionPath string `json:"apiVersionPath" yaml:"apiVersionPath"`
}

type NodeCapacity struct {
	CPU    u.Int32String `json:"cpu" yaml:"cpu"`
	Memory u.Int32String `json:"memory" yaml:"memory"`
	// Maximum number of runners that can be created and stored in State
	// at the same time. This limitation based on Apple Visualization limitations.
	MaxActiveRunners u.Int32String `json:"maxActiveRunners" yaml:"maxActiveRunners"`
}

func GetGithubClientConfig(c *TarterConfig) (ghConfig ghclient.ClientConfig) {
	return ghclient.ClientConfig{
		AppID:          int64(c.GhAppID),
		InstallationID: int64(c.GhAppInstallationID),
		PrivateKey:     []byte(c.GhAppPrivateKey),
		PrivateKeyFile: c.GhAppPrivateKeyFile,
		Organization:   c.GhAppOrg,
	}
}
