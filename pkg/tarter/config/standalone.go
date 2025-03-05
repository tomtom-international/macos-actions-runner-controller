package config

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"gopkg.in/yaml.v3"
	"os"
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

func (c *StandaloneConfig) GetControllerConfig() ControllerConfig {
	return ControllerConfig{}
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
