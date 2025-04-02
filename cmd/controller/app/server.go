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
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/controller/config"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"os"
	"os/signal"
	"syscall"
)

var (
	configuration config.ControllerConfig
)

func NewControllerCommand() *cobra.Command {
	// TODO: check Etcd Connectivity
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "The MacOS Actions Runner Controller is a controller for autoscaling self-hosted GitHub Actions runners on macOS systems.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "version" {
				return nil
			}
			err := config.LoadConfiguration(&configuration)
			if err != nil {
				return errors.New("error: failed to load Controller configuration. " + err.Error())
			}
			// TODO: validate configuration parameters
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			logger.InitLogger(configuration.LogDebug, configuration.LogCaller, configuration.LogStacktrace)
			c, err := NewController()
			if err != nil {
				logger.Fatalf("Error creating Controller: %s", err.Error())
			}
			logger.Infof("Server running on %s", configuration.Port)
			Run(c)
		},
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version of MacOS Actions Runner Controller",
		Run: func(cmd *cobra.Command, args []string) {
			info := coreVersion.GetVersionInfo()
			fmt.Printf("Version: %s\nDate: %s\nCommit SHA: %s\nPlatform: %s\n", info.Version, info.BuildDate, info.GitCommit, info.Platform)
		},
	}
	cmd.AddCommand(versionCmd)

	return cmd
}

func NewController() (*controller.Controller, error) {
	c, err := controller.NewController(configuration)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func Run(c *controller.Controller) {
	logger.Infof("Version: ...")

	// Create a context that is canceled on SIGINT or SIGTERM signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	defer func(EtcdClient *etcd.EtcdClient) {
		logger.Debugf("Closing etcd client")
		err := EtcdClient.Close()
		if err != nil {
			logger.Fatalf(err)
		}
	}(c.EtcdClient)

	go c.RunWithContext(ctx)
	go c.ListenAndServe(configuration)

	// Wait for context cancellation
	<-ctx.Done()
	logger.Infof("Shutting down")
}
