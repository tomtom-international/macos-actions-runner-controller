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
	"github.com/caarlos0/env"
)

type ControllerConfig struct {
	// address is the IP address for the Controller to serve on (default 0.0.0.0
	// for serving on all interfaces)
	Address string `env:"ADDRESS" envDefault:"0.0.0.0"`
	// port is the port for the Controller to serve on.
	Port                   string   `env:"PORT" envDefault:"8043"`
	AwsRegion              string   `env:"AWS_REGION" envDefault:"eu-west-1"`
	AwsRunnerRequestSQSUrl string   `env:"AWS_RUNNER_REQUEST_SQS_URL"`
	CORSAllowedOrigins     string   `env:"CORS_ALLOWED_ORIGINS" envDefault:"*"`
	EtcdEndpoints          []string `env:"ETCD_ENDPOINTS" envDefault:"http://localhost:2379,"`
	// EtcdNodeDeregisterLease is the lease time in seconds for the node deregister
	EtcdNodeDeregisterLease int `env:"ETCD_NODE_POOL_DEREGISTER_LEASE" envDefault:"3600"`
	// EtcdRunnerFinishedLease is the lease time in seconds for the runner finished
	EtcdRunnerFinishedLease int `env:"ETCD_RUNNER_FINISHED_LEASE" envDefault:"172800"`

	EtcdTLSEnabled  bool   `env:"ETCD_TLS_ENABLED" envDefault:"false"`
	EtcdTLSCertFile string `env:"ETCD_TLS_CERT_FILE"`
	EtcdTLSKeyFile  string `env:"ETCD_TLS_KEY_FILE"`
	EtcdTLSCAFile   string `env:"ETCD_TLS_CA_FILE"`

	EtcdUsername string `env:"ETCD_USERNAME"`
	EtcdPassword string `env:"ETCD_PASSWORD"`

	LogDebug      bool `env:"LOG_DEBUG" envDefault:"false"`
	LogCaller     bool `env:"LOG_CALLER" envDefault:"false"`
	LogStacktrace bool `env:"LOG_STACKTRACE" envDefault:"false"`
	LogJSON       bool `env:"LOG_JSON" envDefault:"false"`
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
)
