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

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env"
	"github.com/google/go-github/v61/github"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/sqs"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"gopkg.in/yaml.v3"
)

var (
	cfg          hookConfig
	runnerConfig types.RunnerConfig
	githubClient *ghclient.Client
	offlineCount = 0
	checksPassed = 0
)

// defaultBackoff provides sensible default settings
var defaultBackoff = backoffConfig{
	initialInterval: 1 * time.Second,
	maxInterval:     30 * time.Second,
	maxElapsedTime:  2 * time.Minute,
	multiplier:      2.0,
}

// backoffConfig defines exponential backoff settings
type backoffConfig struct {
	initialInterval time.Duration
	maxInterval     time.Duration
	maxElapsedTime  time.Duration
	multiplier      float64
}

type hookConfig struct {
	AwsRegion                 string `env:"AWS_REGION" envDefault:"eu-west-1"`
	AwsRunnerSQSUrl           string `env:"AWS_RUNNER_SQS_URL"`
	GhaJITConfig              string `env:"ACTIONS_RUNNER_INPUT_JITCONFIG"`
	RunnerConfigFile          string `env:"RUNNER_CONFIG_FILE" envDefault:"/opt/tarter-hook/runner_config.yaml"`
	GhaRunnerGroup            string `env:"GHA_RUNNER_GROUP"`
	GhAppPrivateKey           string `env:"GH_APP_PRIVATE_KEY"`
	GhAppOrg                  string `env:"GH_APP_ORG"`
	GhAppID                   int    `env:"GH_APP_ID"`
	GhAppInstallationID       int    `env:"GH_APP_INSTALLATION_ID"`
	RunnerStatusCheckPeriod   int    `env:"RUNNER_STATUS_CHECK_PERIOD" envDefault:"60"`
	RunnerFinishedChecksCount int    `env:"RUNNER_FINISHED_CHECKS_COUNT" envDefault:"1"`
	RunnerOfflineChecksCount  int    `env:"RUNNER_OFFLINE_CHECKS_COUNT" envDefault:"15"`
}

// loadHookConfiguration loads the configuration from environment variables
func loadHookConfiguration() {
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("error: failed to load configuration. %v", err)
	}
}

// readRunnerConfig reads the runner configuration from the specified file
func readRunnerConfig() {
	data, err := os.ReadFile(cfg.RunnerConfigFile)
	if err != nil {
		log.Fatalf("error: failed to read runner configuration. %v", err)
	}
	err = yaml.Unmarshal(data, &runnerConfig)
	if err != nil {
		log.Fatalf("error: failed to unmarshal runner configuration. %v", err)
	}
}

// initGithubClient initializes the GitHub client with the provided configuration
func initGithubClient() {
	ghConfig := ghclient.ClientConfig{
		AppID:          int64(cfg.GhAppID),
		InstallationID: int64(cfg.GhAppInstallationID),
		PrivateKey:     []byte(cfg.GhAppPrivateKey),
		Organization:   cfg.GhAppOrg,
	}
	var err error
	githubClient, err = ghclient.New(ghConfig)
	if err != nil {
		log.Fatalf("Failed to create github client: %v", err)
	}
}

func main() {
	info := coreVersion.GetVersionInfo()
	log.Printf("Starting hook application\nVersion: %s\nDate: %s\nCommit SHA: %s\nPlatform: %s\n",
		info.Version, info.BuildDate, info.GitCommit, info.Platform)
	log.Println("Reading configuration")
	loadHookConfiguration()
	readRunnerConfig()

	log.Println("Initializing github client")
	initGithubClient()

	log.Println("Extracting gha runner name")
	ghaRunnerName, err := extractGhaName(cfg.GhaJITConfig)
	if err != nil {
		log.Fatalf("Failed to extract gha runner name: %v", err)
	}

	log.Printf("Gha runner name: %s", ghaRunnerName)

	runnerConfig.JitConfig = cfg.GhaJITConfig
	runnerConfig.Name = cfg.GhaRunnerGroup
	runnerMessage := types.Runner{
		GhaRunnerName: ghaRunnerName,
		Config:        runnerConfig,
	}
	message, err := json.Marshal(runnerMessage)
	if err != nil {
		log.Fatalf("Failed to marshal runner config: %v", err)
	}

	sendMessageToSqs(string(message))
	log.Println("Runner sent to sqs")

	log.Println("Waiting for runner to finish")
	time.Sleep(15 * time.Second)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	interval := time.Duration(cfg.RunnerStatusCheckPeriod) * time.Second
	ticker := time.NewTicker(interval)
	defer func() {
		stop()
		ticker.Stop()
	}()

runnerStatusCheckLoop:
	for checkStatus(ghaRunnerName) {
		select {
		case <-ctx.Done():
			break runnerStatusCheckLoop
		case <-ticker.C:
		}
	}

	log.Println("Hook finished")
}

// checkStatus checks the status of the GHA runner and returns true if the runner is online
// and false if it is offline. It also increments the offline count and checks if it exceeds
func checkStatus(ghaRunnerName string) bool {
	var runners *github.Runners
	var err error
	// Use context with timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = withRetry(ctx, defaultBackoff, func() error {
		runners, err = githubClient.GetRunnerByName(ghaRunnerName)
		return err
	})

	if err != nil {
		log.Printf("Failed to get runner %s from github after retries: %v", ghaRunnerName, err)
		return true
	}

	if len(runners.Runners) > 0 {
		if *runners.Runners[0].Status == "online" {
			log.Printf("Runner %s is active", ghaRunnerName)
			return true
		} else {
			log.Printf("Runner %s is offline", ghaRunnerName)
			offlineCount++
			if offlineCount > cfg.RunnerOfflineChecksCount {
				log.Printf("Runner %s failed to start in secifed interval", ghaRunnerName)
				return false
			}
			return true
		}
	}
	if checksPassed < cfg.RunnerFinishedChecksCount {
		checksPassed++
		log.Printf("Runner %s finished.", ghaRunnerName)
		return true
	}
	return false
}

// sendMessageToSqs sends a message to the SQS queue with runner configuration
// to be consumed by MacOS Actions Runner Controller
func sendMessageToSqs(message string) {
	client, err := sqs.NewClient(
		&sqs.SQSConfig{
			QueueURL:  cfg.AwsRunnerSQSUrl,
			AWSRegion: cfg.AwsRegion,
		},
	)
	if err != nil {
		log.Fatalf("Failed to create sqs client: %v", err)
	}

	_, err = client.SendMessage(context.Background(), message)
	if err != nil {
		log.Fatalf("Failed to send message to sqs: %v", err)
	}
}

// extractGhaName decodes the JIT config and extracts the GHA runner settings
func extractGhaName(encodedConf string) (string, error) {
	type jitConfig struct {
		Runner string `json:".runner"`
	}
	type runner struct {
		AgentName string `json:"AgentName"`
	}
	var decodedJitConfig jitConfig
	var decodedRunner runner

	data, err := base64.StdEncoding.DecodeString(encodedConf)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &decodedJitConfig)
	if err != nil {
		return "", err
	}
	data, err = base64.StdEncoding.DecodeString(decodedJitConfig.Runner)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &decodedRunner)
	if err != nil {
		return "", err
	}

	return decodedRunner.AgentName, nil
}

// withRetry executes a function with exponential backoff
// It check if error is a GitHub rate limit error and apply jitter to avoid thundering herd
func withRetry(ctx context.Context, backoffCfg backoffConfig, fn func() error) error {
	backoffInterval := backoffCfg.initialInterval
	startTime := time.Now()

	for {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if error is a GitHub rate limit error
		if isRateLimitError(err) {
			log.Printf("Hit GitHub rate limit, backing off: %v", err)
		} else if !isRetryableError(err) {
			return err
		}

		// Check if we've exceeded the maximum elapsed time
		elapsedTime := time.Since(startTime)
		if elapsedTime >= backoffCfg.maxElapsedTime {
			return fmt.Errorf("operation failed after %v: %w", elapsedTime, err)
		}

		jitter := time.Duration(float64(backoffInterval) * (0.5 + rand.Float64()/2.0))
		log.Printf("Retrying in %v: %v", jitter, err)

		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return ctx.Err()
		}

		// Increase backoff interval for next attempt
		backoffInterval = time.Duration(float64(backoffInterval) * backoffCfg.multiplier)
		if backoffInterval > backoffCfg.maxInterval {
			backoffInterval = backoffCfg.maxInterval
		}
	}
}

// isRateLimitError checks if an error is a GitHub rate limit error
func isRateLimitError(err error) bool {
	return strings.Contains(err.Error(), "rate limit") ||
		strings.Contains(err.Error(), "403 Forbidden") ||
		strings.Contains(err.Error(), "429 Too Many Requests")
}

// isRetryableError checks if an error is worth retrying.
// Consider network errors, timeouts, and 5xx errors as retryable
func isRetryableError(err error) bool {
	return strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection") ||
		strings.Contains(err.Error(), "5") // 5xx errors
}
