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

type ScalingGroup struct {
	Name      string     `yaml:"name"`
	Schedules []Schedule `yaml:"schedules"`
}

type Schedule struct {
	Name            string `yaml:"name"`
	Cron            string `yaml:"cron"`
	DesiredCapacity int    `yaml:"desiredCapacity"`
}

type Config struct {
	ScalingGroups []ScalingGroup `yaml:"scalingGroups"`
}

type ScalerConfig struct {
	MacosRunnerControllerURL        string   `env:"MACOS_RUNNER_CONTROLLER_URL"`
	MacosRunnerControllerAPIVersion string   `env:"MACOS_RUNNER_CONTROLLER_API_VERSION" envDefault:"/api/v1"`
	Port                            string   `env:"PORT" envDefault:"8045"`
	ConfigPath                      string   `env:"CONFIG_PATH"`
	AwsRegion                       string   `env:"AWS_REGION" envDefault:"eu-west-1"`
	ScalerSqsQueueURL               string   `env:"AWS_SCALER_SQS_QUEUE_URL"`
	Address                         string   `env:"ADDRESS" envDefault:"0.0.0.0"`
	EtcdEndpoints                   []string `env:"ETCD_ENDPOINTS" envDefault:"http://localhost:2379,"`
	MaxTerminationRetryCount        int      `env:"MAX_TERMINATION_RETRY_COUNT" envDefault:"12"`

	EtcdTLSEnabled  bool   `env:"ETCD_TLS_ENABLED" envDefault:"false"`
	EtcdTLSCertFile string `env:"ETCD_TLS_CERT_FILE"`
	EtcdTLSKeyFile  string `env:"ETCD_TLS_KEY_FILE"`
	EtcdTLSCAFile   string `env:"ETCD_TLS_CA_FILE"`

	EtcdUsername string `env:"ETCD_USERNAME"`
	EtcdPassword string `env:"ETCD_PASSWORD"`

	LogJSON       bool `env:"LOG_JSON" envDefault:"false"`
	LogCaller     bool `env:"LOG_CALLER" envDefault:"false"`
	LogStacktrace bool `env:"LOG_STACKTRACE" envDefault:"false"`
	LogDebug      bool `env:"LOG_DEBUG" envDefault:"false"`
	DryRun        bool `env:"SCALER_DRY_RUN" envDefault:"false"`
}
