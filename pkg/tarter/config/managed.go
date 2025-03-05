package config

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"gopkg.in/yaml.v3"
	"os"
)

type ManagedConfig struct {
	Runners      []types.RunnerConfig `yaml:"runners"`
	Controller   ControllerConfig     `yaml:"controller"`
	NodeCapacity NodeCapacity         `yaml:"nodeCapacity"`
}

func (c *ManagedConfig) ReadConfig(path string) error {
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

func (c *ManagedConfig) GetRunnersConfig() []types.RunnerConfig {
	return c.Runners
}

func (c *ManagedConfig) GetControllerConfig() ControllerConfig {
	return c.Controller
}

func (c *ManagedConfig) GetNodeCapacity() NodeCapacity {
	return c.NodeCapacity
}

func ReadManagedConfiguration(configPath string) ManagedConfig {
	var config ManagedConfig
	err := config.ReadConfig(configPath)
	if err != nil {
		logger.Errorf("Failed to read Tarter ManagedConfig configuration. Error: %v", err)
		os.Exit(1)
	}

	return config
}
