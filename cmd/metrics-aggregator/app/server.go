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
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env"
	"github.com/spf13/cobra"
	ma "github.com/tomtom-international/macos-actions-runner-controller/internal/metrics-aggregator"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/metrics-aggregator/api"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/metrics-aggregator/config"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

var (
	cfg config.MetricsAggregatorConfig
)

func NewMetricsAggregatorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics-aggregator",
		Short: "The MacOS Tart VMs metrics aggregator service",
		Long:  `A service that aggregates metrics from exporters installed on all running tart vm and adds GHA runner labels`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "version" {
				return nil
			}

			// Load configuration from environment variables
			if err := loadConfiguration(&cfg); err != nil {
				return errors.New("error: failed to load configuration from environment. " + err.Error())
			}

			// Validate and update config with flags
			if err := validateConfig(&cfg); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := logger.InitLogger(cfg.LogDebug, cfg.LogCaller, cfg.LogStacktrace, cfg.LogJSON)
			if err != nil {
				return err
			}

			if err := Run(); err != nil {
				logger.Errorf("Error running Metrics Aggregator: %s", err.Error())
				return err
			}
			return nil
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of MacOS Tart VMs metrics aggregator service",
		Run: func(cmd *cobra.Command, args []string) {
			info := coreVersion.GetVersionInfo()
			fmt.Printf("Version: %s\nDate: %s\nCommit SHA: %s\nPlatform: %s\n", info.Version, info.BuildDate, info.GitCommit, info.Platform)
		},
	}
	cmd.AddCommand(versionCmd)

	return cmd
}

func Run() error {
	defer logger.Sync()
	logger.Infof("Starting Metrics Metrics Aggregator...")
	logger.Infof("Version: %s", coreVersion.GetVersionInfo().Version)

	// Create a context that is canceled on SIGINT or SIGTERM signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	e, err := ma.NewMetricsAggregator(cfg.TarterURL, cfg.TartPath, cfg.ScrapePort)
	if err != nil {
		return fmt.Errorf("failed to create MetricsAggregator: %w", err)
	}

	// Start the HTTP server
	address := net.ParseIP(cfg.Address)
	server := api.NewServer(e, address, cfg.Port)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("failed to start and HTTP server: %s", err.Error())
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	logger.Infof("Metrics Aggregator stopped")
	return nil
}

func loadConfiguration(c *config.MetricsAggregatorConfig) error {
	if err := env.Parse(c); err != nil {
		return err
	}
	return nil
}

func validateConfig(c *config.MetricsAggregatorConfig) error {
	if c.TarterURL == "" {
		return errors.New("error: Tarter URL is required. Use TARTER_URL environment variable")
	}
	return nil
}
