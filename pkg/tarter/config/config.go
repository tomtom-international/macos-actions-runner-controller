package config

import (
	"github.com/caarlos0/env"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/secretsmanager"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	u "github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

const (
	StandaloneMode TarterMode = "standalone"
	ManagedMode    TarterMode = "managed"
)

type TarterMode string

type TarterConfig struct {
	LogDebug           bool   `env:"LOG_DEBUG" envDefault:"false"`
	LogCaller          bool   `env:"LOG_CALLER" envDefault:"false"`
	LogStacktrace      bool   `env:"LOG_STACKTRACE" envDefault:"false"`
	Port               string `env:"PORT" envDefault:"8041"`
	Mode               string `env:"TARTER_MODE"`
	ConfigFolder       string `env:"TARTER_CONFIG_PATH" envDefault:"/opt/tarter/config"`
	TartPath           string `env:"TART_PATH" envDefault:"/opt/homebrew/bin/tart"`
	AwsRegion          string `env:"AWS_REGION" envDefault:"eu-west-1"`
	AwsSecretGitHubApp string `env:"AWS_SM_GH_APP" envDefault:"stage/gha/github-app"`
}

type Config interface {
	GetRunnersConfig() []types.RunnerConfig
	GetControllerConfig() ControllerConfig
	GetNodeCapacity() NodeCapacity
	ReadConfig(path string) error
}

type ControllerConfig struct {
	Server         string `json:"server" yaml:"server"`
	ApiVersionPath string `json:"apiVersionPath" yaml:"apiVersionPath"`
}

type NodeCapacity struct {
	Cpu    u.Int32String `json:"cpu"  yaml:"cpu"`
	Memory u.Int32String `json:"memory" yaml:"memory"`
	// Maximum number of runners that can be created and stored in State
	// at the same time. This limitation based on Apple Visualisation limitations.
	MaxActiveRunners u.Int32String `json:"maxActiveRunners" yaml:"maxActiveRunners"`
}

func LoadTarterConfiguration(c *TarterConfig) error {
	if err := env.Parse(c); err != nil {
		return err
	}
	return nil
}

func InitSecretManager(c *TarterConfig) (*secretsmanager.SCM, error) {
	smc, err := secretsmanager.New(c.AwsRegion)
	if err != nil {
		return nil, err
	}
	return smc, nil
}
