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
	"sync"
	"syscall"

	"github.com/caarlos0/env"
	"github.com/spf13/cobra"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/api"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/config"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"gopkg.in/yaml.v3"
)

var (
	cfg config.ScalerConfig
)

func NewScalerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaler",
		Short: "The MacOS Actions Runner node pool scaling service",
		Long:  `A service that scales MacOS Actions Runner node pool by schedule provided in configuration.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "version" {
				return nil
			}

			// Load configuration from environment variables
			if err := loadConfiguration(&cfg); err != nil {
				return errors.New("error: failed to load configuration from environment. " + err.Error())
			}

			// Validate and update config with flags
			if err := validateConfig(&cfg, cmd); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := logger.InitLogger(cfg.LogDebug, cfg.LogCaller, cfg.LogStacktrace, cfg.LogJSON)
			if err != nil {
				return err
			}
			scalingCfg, err := readConfig(cfg.ConfigPath)
			if err != nil {
				return err
			}
			logger.Debugf("Starting Scaler with configuration %+v", scalingCfg)

			if err := Run(scalingCfg); err != nil {
				logger.Errorf("Error running Scaler: %s", err.Error())
				return err
			}
			return nil
		},
	}

	cmd.PersistentFlags().String("config", "", "Path to the configuration file")
	cmd.PersistentFlags().Bool("dry-run", false, "Enable dry-run mode")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of MacOS Actions Runner Scaler",
		Run: func(cmd *cobra.Command, args []string) {
			info := coreVersion.GetVersionInfo()
			fmt.Printf("Version: %s\nDate: %s\nCommit SHA: %s\nPlatform: %s\n", info.Version, info.BuildDate, info.GitCommit, info.Platform)
		},
	}
	cmd.AddCommand(versionCmd)

	return cmd
}

func Run(scalingCfg *config.Config) error {
	defer logger.Sync()
	logger.Infof("Starting Scaler...")
	logger.Infof("Version: %s", coreVersion.GetVersionInfo().Version)

	// Create a context that is canceled on SIGINT or SIGTERM signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := scaler.NewScaler(cfg, scalingCfg)
	if err != nil {
		return fmt.Errorf("failed to create scaler: %w", err)
	}
	defer s.Stop()

	// Start the scaler
	var wg sync.WaitGroup
	if err := s.Start(ctx, &wg); err != nil {
		return fmt.Errorf("failed to run scaler: %w", err)
	}

	// Start the HTTP server
	address := net.ParseIP(cfg.Address)
	server := api.NewServer(s, address, cfg.Port)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("failed to start and HTTP server: %s", err.Error())
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	logger.Infof("Starting graceful shutdown...")

	wg.Wait()
	logger.Infof("All operations completed, shutting down")
	return nil
}

func readConfig(path string) (*config.Config, error) {
	// Check if the config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found at path: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var c config.Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &c, nil
}

// validateConfig validates configuration and updates with flag values
func validateConfig(c *config.ScalerConfig, cmd *cobra.Command) error {
	// Check if ConfigPath and DryRun were provided via flag (overrides env var)
	if configFlag := cmd.Flags().Lookup("config"); configFlag != nil && configFlag.Changed {
		c.ConfigPath = configFlag.Value.String()
	}
	if configFlag := cmd.Flags().Lookup("dry-run"); configFlag != nil && configFlag.Changed {
		c.ConfigPath = configFlag.Value.String()
	}

	// Verify config path is set
	if c.ConfigPath == "" {
		return errors.New("error: config path is required. Use --config flag or CONFIG_PATH environment variable")
	}

	return nil
}

func loadConfiguration(c *config.ScalerConfig) error {
	if err := env.Parse(c); err != nil {
		return err
	}
	return nil
}
