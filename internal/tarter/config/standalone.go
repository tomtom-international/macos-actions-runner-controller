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
	"os"

	controllerClient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"gopkg.in/yaml.v3"
)

type StandaloneConfig struct {
	Runners      []types.RunnerConfig `yaml:"runners"`
	NodeCapacity NodeCapacity         `yaml:"nodeCapacity"`
}

func (c *StandaloneConfig) ReadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return err
	}

	return nil
}

func (c *StandaloneConfig) GetRunnersConfig() []types.RunnerConfig {
	return c.Runners
}

func (c *StandaloneConfig) GetControllerConfig() controllerClient.ClientConfig {
	return controllerClient.ClientConfig{}
}

func (c *StandaloneConfig) GetNodeCapacity() NodeCapacity {
	return c.NodeCapacity
}

func ReadStandaloneConfiguration(configPath string) StandaloneConfig {
	var config StandaloneConfig
	err := config.ReadConfig(configPath)
	if err != nil {
		logger.Errorf("Failed to read Tarter Standalone configuration. Error: %v", err)
		os.Exit(1)
	}

	return config
}
