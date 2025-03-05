package config

import (
	"github.com/caarlos0/env"
)

type ControllerConfig struct {
	LogDebug      bool `env:"LOG_DEBUG" envDefault:"false"`
	LogCaller     bool `env:"LOG_CALLER" envDefault:"false"`
	LogStacktrace bool `env:"LOG_STACKTRACE" envDefault:"false"`
	// address is the IP address for the Controller to serve on (default 0.0.0.0
	// for serving on all interfaces)
	Address string `env:"ADDRESS" envDefault:"0.0.0.0"`
	// port is the port for the Controller to serve on.
	Port                   string   `env:"PORT" envDefault:"8043"`
	EtcdEndpoints          []string `env:"ETCD_ENDPOINTS" envDefault:"http://localhost:2379,"`
	AwsRegion              string   `env:"AWS_REGION" envDefault:"eu-west-1"`
	AwsRunnerRequestSQSUrl string   `env:"AWS_RUNNER_REQUEST_SQS_URL"`

	// EtcdNodeDeregisterLease is the lease time in seconds for the node deregister
	EtcdNodeDeregisterLease int `env:"ETCD_NODE_POOL_DEREGISTER_LEASE" envDefault:"3600"`
	// EtcdRunnerFinishedLease is the lease time in seconds for the runner finished
	EtcdRunnerFinishedLease int `env:"ETCD_NODE_POOL_DEREGISTER_LEASE" envDefault:"172800"`
}

func LoadConfiguration(c *ControllerConfig) error {
	if err := env.Parse(c); err != nil {
		return err
	}
	return nil
}

const (
	EtcdNodePoolKey = "nodes/"
	EtcdRunnersKey  = "runners/"
	EtcdEventsKey   = "events/"
)
