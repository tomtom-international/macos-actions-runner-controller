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

package app

import (
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	coreApi "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/config"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"net/http"
)

var (
	configFile   string
	mode         string
	tarterConfig config.TarterConfig
)

func NewTarterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tarter",
		Short: "Tarter is a service that manages the lifecycle of Tart runners",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "version" {
				return nil
			}
			err := config.LoadTarterConfiguration(&tarterConfig)
			if err != nil {
				return errors.New("error: failed to load Tarter configuration. " + err.Error())
			}
			if configFile == "" {
				return errors.New("error: --config argument is required")
			}
			if mode == "" && tarterConfig.Mode == "" {
				return errors.New("error: --mode argument is required. Or set TARTER_MODE environment variable")
			}
			if mode != "" {
				tarterConfig.Mode = mode
			}
			if tarterConfig.Mode != "standalone" && tarterConfig.Mode != "managed" {
				return errors.New("error: invalid mode. Must be 'standalone' or 'managed'")
			}
			// TODO: Add default values for liveness and startup probes
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {

			logger.InitLogger(tarterConfig.LogDebug, tarterConfig.LogCaller, tarterConfig.LogStacktrace)
			t, r, err := NewTarter()
			if err != nil {
				logger.Fatalf("Error creating Tarter application: %s", err.Error())
			}

			logger.Infof("Server running on %s", tarterConfig.Port)
			logger.Fatalf(Run(r, t))
		},
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version of Tarter",
		Run: func(cmd *cobra.Command, args []string) {
			info := coreVersion.GetVersionInfo()
			fmt.Printf("Version: %s\nDate: %s\nCommit SHA: %s\nPlatform: %s\n", info.Version, info.BuildDate, info.GitCommit, info.Platform)
		},
	}
	cmd.AddCommand(versionCmd)

	cmd.Flags().StringVar(&configFile, "config", "", "Path to the configuration file (required)")
	cmd.Flags().StringVar(&mode, "mode", "standalone", "Mode of operation: standalone or managed (optional)")

	return cmd
}

func NewTarter() (*tarter.Tarter, *mux.Router, error) {
	versionInfo := coreVersion.GetVersionInfo()
	nodeName, err := utils.GetHostname()
	if err != nil {
		return nil, nil, err
	}
	nodeIp, err := utils.GetNodeIP()
	if err != nil {
		return nil, nil, err
	}
	var appConfig config.Config
	switch {
	case config.TarterMode(tarterConfig.Mode) == config.StandaloneMode:
		standaloneConfig := config.ReadStandaloneConfiguration(configFile)
		appConfig = &standaloneConfig
	case config.TarterMode(tarterConfig.Mode) == config.ManagedMode:
		managedConfig := config.ReadManagedConfiguration(configFile)
		appConfig = &managedConfig
	}
	t, err := tarter.NewTarter(tarterConfig, appConfig, configFile, nodeName, nodeIp, versionInfo)
	if err != nil {
		return nil, nil, err
	}
	r := mux.NewRouter()

	coreApi.SetupGenericHandlers(r)
	api.SetupRoutes(r, t.StateManager)

	// These routes are only for development purposes
	api.SetupSysRoutes(r, t)

	return t, r, nil
}

func Run(r *mux.Router, t *tarter.Tarter) error {
	switch {
	case config.TarterMode(tarterConfig.Mode) == config.StandaloneMode:
		api.SetupStandaloneTarterRoutes(r, t)
		t.StartStandaloneTarter()
	case config.TarterMode(tarterConfig.Mode) == config.ManagedMode:
		t.StartManagedTarter()
	}
	return http.ListenAndServe(fmt.Sprintf(":%s", tarterConfig.Port), r)
}
