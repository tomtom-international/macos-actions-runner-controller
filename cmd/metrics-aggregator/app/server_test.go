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
	"github.com/tomtom-international/macos-actions-runner-controller/internal/metrics-aggregator/config"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsAggregatorCommand(t *testing.T) {
	t.Run("should create command with expected subcommands", func(t *testing.T) {
		cmd := NewMetricsAggregatorCommand()

		assert.Equal(t, "metrics-aggregator", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)

		// Check for version subcommand
		versionCmd, _, err := cmd.Find([]string{"version"})
		require.NoError(t, err)
		assert.Equal(t, "version", versionCmd.Use)
	})
}

// TestEnvironmentVariables tests loading log config from env vars
func TestEnvironmentVariables(t *testing.T) {
	origLogDebug := os.Getenv("LOG_DEBUG")
	origLogCaller := os.Getenv("LOG_CALLER")
	origLogStacktrace := os.Getenv("LOG_STACKTRACE")
	origLogJSON := os.Getenv("LOG_JSON")
	origTarterURL := os.Getenv("TARTER_URL")

	// Restore environment variables after test
	defer func() {
		os.Setenv("LOG_DEBUG", origLogDebug)
		os.Setenv("LOG_CALLER", origLogCaller)
		os.Setenv("LOG_STACKTRACE", origLogStacktrace)
		os.Setenv("LOG_JSON", origLogJSON)
		os.Setenv("TARTER_URL", origTarterURL)
	}()

	t.Run("should load default values when environment variables not set", func(t *testing.T) {
		os.Unsetenv("LOG_DEBUG")
		os.Unsetenv("LOG_CALLER")
		os.Unsetenv("LOG_STACKTRACE")
		os.Unsetenv("LOG_JSON")
		os.Unsetenv("TARTER_URL")

		var testCfg config.MetricsAggregatorConfig
		err := loadConfiguration(&testCfg)

		require.NoError(t, err)
		assert.False(t, testCfg.LogDebug)
		assert.False(t, testCfg.LogCaller)
		assert.False(t, testCfg.LogStacktrace)
		assert.False(t, testCfg.LogJSON)
		assert.Empty(t, testCfg.TarterURL)
	})

	t.Run("should load values from environment variables", func(t *testing.T) {
		os.Setenv("LOG_DEBUG", "true")
		os.Setenv("LOG_CALLER", "true")
		os.Setenv("LOG_STACKTRACE", "true")
		os.Setenv("LOG_JSON", "true")
		os.Setenv("TARTER_URL", "http://localhost:8080")

		var testCfg config.MetricsAggregatorConfig
		err := loadConfiguration(&testCfg)

		require.NoError(t, err)
		assert.True(t, testCfg.LogDebug)
		assert.True(t, testCfg.LogCaller)
		assert.True(t, testCfg.LogStacktrace)
		assert.True(t, testCfg.LogJSON)
		assert.Equal(t, "http://localhost:8080", testCfg.TarterURL)
	})
}

func TestRunE(t *testing.T) {

	t.Run("should return error when tarter URL is empty", func(t *testing.T) {
		cmd := NewMetricsAggregatorCommand()
		preRunE := cmd.PersistentPreRunE

		err := preRunE(cmd, []string{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TARTER_URL")
	})
}
