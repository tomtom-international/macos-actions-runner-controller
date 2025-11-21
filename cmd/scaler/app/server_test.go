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
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/config"
	"gopkg.in/yaml.v3"
)

// TestNewScalerCommand tests command creation
func TestNewScalerCommand(t *testing.T) {
	t.Run("should create command with expected subcommands", func(t *testing.T) {
		cmd := NewScalerCommand()

		assert.Equal(t, "scaler", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)

		// Check for version subcommand
		versionCmd, _, err := cmd.Find([]string{"version"})
		require.NoError(t, err)
		assert.Equal(t, "version", versionCmd.Use)
	})
}

// TestConfigPathHandling tests config path handling from flags and env vars
func TestConfigPathHandling(t *testing.T) {
	// Save original env var and restore after test
	origConfigPath := os.Getenv("CONFIG_PATH")
	defer os.Setenv("CONFIG_PATH", origConfigPath)

	t.Run("should return error when config path is not provided", func(t *testing.T) {
		os.Unsetenv("CONFIG_PATH")

		testCfg := config.ScalerConfig{}
		err := loadConfiguration(&testCfg)
		require.NoError(t, err)

		cmd := &cobra.Command{}

		err = validateConfig(&testCfg, cmd)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "config path is required")
	})

	t.Run("should set config path from flag", func(t *testing.T) {
		os.Unsetenv("CONFIG_PATH")

		testCfg := config.ScalerConfig{}

		cmd := &cobra.Command{}
		cmd.Flags().String("config", "", "Path to config")
		err := cmd.Flags().Set("config", "/path/from/flag")
		require.NoError(t, err)

		err = validateConfig(&testCfg, cmd)

		require.NoError(t, err)
		assert.Equal(t, "/path/from/flag", testCfg.ConfigPath)
	})

	t.Run("should set config path from environment variable", func(t *testing.T) {
		os.Setenv("CONFIG_PATH", "/path/from/env")

		testCfg := config.ScalerConfig{}
		err := loadConfiguration(&testCfg)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		cmd.Flags().String("config", "", "Path to config")

		err = validateConfig(&testCfg, cmd)

		require.NoError(t, err)
		assert.Equal(t, "/path/from/env", testCfg.ConfigPath)
	})

	t.Run("flag should override environment variable", func(t *testing.T) {
		os.Setenv("CONFIG_PATH", "/path/from/env")

		testCfg := config.ScalerConfig{}
		err := loadConfiguration(&testCfg)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		cmd.Flags().String("config", "", "Path to config")
		err = cmd.Flags().Set("config", "/path/from/flag")
		require.NoError(t, err)

		err = validateConfig(&testCfg, cmd)

		require.NoError(t, err)
		assert.Equal(t, "/path/from/flag", testCfg.ConfigPath)
	})
}

// TestReadConfig tests reading the YAML config file
func TestReadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	validConfigPath := filepath.Join(tmpDir, "valid-config.yaml")
	invalidConfigPath := filepath.Join(tmpDir, "invalid-config.yaml")
	nonExistentPath := filepath.Join(tmpDir, "non-existent.yaml")

	validConfig := config.Config{
		ScalingGroups: []config.ScalingGroup{
			{Name: "group-1", Schedules: []config.Schedule{{Name: "schedule-1", Cron: "0 0 * * sat-sun", DesiredCapacity: 15}}},
			{Name: "group-2", Schedules: []config.Schedule{{Name: "schedule-2", Cron: "0 0 * * *", DesiredCapacity: 1}}},
		},
	}

	validData, err := yaml.Marshal(validConfig)
	require.NoError(t, err)
	err = os.WriteFile(validConfigPath, validData, 0644)
	require.NoError(t, err)

	// Create an invalid config (invalid YAML)
	err = os.WriteFile(invalidConfigPath, []byte("invalid: yaml: ["), 0644)
	require.NoError(t, err)

	t.Run("should successfully read valid config", func(t *testing.T) {
		cfg, err := readConfig(validConfigPath)

		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Len(t, cfg.ScalingGroups, 2)
		assert.Equal(t, "group-1", cfg.ScalingGroups[0].Name)
		assert.Equal(t, "group-2", cfg.ScalingGroups[1].Name)
		assert.Equal(t, 15, cfg.ScalingGroups[0].Schedules[0].DesiredCapacity)
		assert.Equal(t, 1, cfg.ScalingGroups[1].Schedules[0].DesiredCapacity)
	})

	t.Run("should return error for non-existent config file", func(t *testing.T) {
		cfg, err := readConfig(nonExistentPath)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, cfg)
	})

	t.Run("should return error for invalid YAML", func(t *testing.T) {
		cfg, err := readConfig(invalidConfigPath)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
		assert.Nil(t, cfg)
	})
}

// TestEnvironmentVariables tests loading log config from env vars
func TestEnvironmentVariables(t *testing.T) {
	// Save original environment variables
	origLogDebug := os.Getenv("LOG_DEBUG")
	origLogCaller := os.Getenv("LOG_CALLER")
	origLogStacktrace := os.Getenv("LOG_STACKTRACE")
	origLogJSON := os.Getenv("LOG_JSON")

	// Restore environment variables after test
	defer func() {
		os.Setenv("LOG_DEBUG", origLogDebug)
		os.Setenv("LOG_CALLER", origLogCaller)
		os.Setenv("LOG_STACKTRACE", origLogStacktrace)
		os.Setenv("LOG_JSON", origLogJSON)
	}()

	t.Run("should load default values when environment variables not set", func(t *testing.T) {
		os.Unsetenv("LOG_DEBUG")
		os.Unsetenv("LOG_CALLER")
		os.Unsetenv("LOG_STACKTRACE")
		os.Unsetenv("LOG_JSON")

		var testCfg config.ScalerConfig
		err := loadConfiguration(&testCfg)

		require.NoError(t, err)
		assert.False(t, testCfg.LogDebug)
		assert.False(t, testCfg.LogCaller)
		assert.False(t, testCfg.LogStacktrace)
		assert.False(t, testCfg.LogJSON)
		assert.Empty(t, testCfg.ConfigPath)
	})

	t.Run("should load values from environment variables", func(t *testing.T) {
		os.Setenv("LOG_DEBUG", "true")
		os.Setenv("LOG_CALLER", "true")
		os.Setenv("LOG_STACKTRACE", "true")
		os.Setenv("LOG_JSON", "true")
		os.Setenv("CONFIG_PATH", "/some/path")

		var testCfg config.ScalerConfig
		err := loadConfiguration(&testCfg)

		require.NoError(t, err)
		assert.True(t, testCfg.LogDebug)
		assert.True(t, testCfg.LogCaller)
		assert.True(t, testCfg.LogStacktrace)
		assert.True(t, testCfg.LogJSON)
		assert.Equal(t, "/some/path", testCfg.ConfigPath)
	})
}

func TestRunE(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	validConfig := config.Config{
		ScalingGroups: []config.ScalingGroup{
			{Name: "group-1", Schedules: []config.Schedule{{Name: "schedule-1", Cron: "0 0 * * sat-sun", DesiredCapacity: 15}}},
		},
	}
	validData, err := yaml.Marshal(validConfig)
	require.NoError(t, err)
	err = os.WriteFile(configPath, validData, 0644)
	require.NoError(t, err)

	t.Run("should return error when config file cannot be read", func(t *testing.T) {
		cmd := NewScalerCommand()

		cfg.ConfigPath = filepath.Join(tmpDir, "non-existent.yaml")

		runE := cmd.RunE

		err := runE(cmd, []string{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// For integration test
/*
func TestCommandIntegration(t *testing.T) {
	// Only run this test when a specific flag is set, as it modifies RunE
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a valid config
	validConfig := Config{
		Schedules: []Schedule{
			{GroupName: "group-1", Schedule: "0 0 * * sat-sun", MinSize: 10},
		},
	}

	validData, err := yaml.Marshal(validConfig)
	require.NoError(t, err)
	err = os.WriteFile(configPath, validData, 0644)
	require.NoError(t, err)

	// Create command
	cmd := NewScalerCommand()

	// Replace RunE to avoid hanging
	originalRunE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Call the original function's logic up until the select{}
		err := originalRunE(cmd, args)
		// If an error occurred, return it
		if err != nil {
			return err
		}
		// Instead of hanging, return nil
		return nil
	}

	// Set the flag
	err = cmd.Flags().Set("config", configPath)
	require.NoError(t, err)

	// Execute the command
	err = cmd.Execute()
	require.NoError(t, err)
}
*/
