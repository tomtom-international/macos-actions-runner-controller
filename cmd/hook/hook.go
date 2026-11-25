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
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env"
	"github.com/google/go-github/v61/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/sqs"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"gopkg.in/yaml.v3"
)

var (
	cfg          hookConfig
	runnerConfig types.RunnerConfig
)

type runnerAPI interface {
	GetRunnerByName(runnerName string) (*github.Runners, error)
	RemoveRunner(runnerID int64) error
}

type loopExit int

const (
	exitFinished loopExit = iota
	exitOfflineLimit
	exitSignal
)

func (e loopExit) String() string {
	switch e {
	case exitFinished:
		return "runner finished"
	case exitOfflineLimit:
		return "runner offline limit reached"
	case exitSignal:
		return "terminated by signal"
	}
	return fmt.Sprintf("loopExit(%d)", int(e))
}

type statusWatcher struct {
	api            runnerAPI
	name           string
	backoff        backoffConfig
	offlineLimit   int
	finishedChecks int
	offlineCount   int
	checksPassed   int
}

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
	RunnerDeregisterTimeout   int    `env:"RUNNER_DEREGISTER_TIMEOUT" envDefault:"15"`
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
func initGithubClient() *ghclient.Client {
	ghConfig := ghclient.ClientConfig{
		AppID:          int64(cfg.GhAppID),
		InstallationID: int64(cfg.GhAppInstallationID),
		PrivateKey:     []byte(cfg.GhAppPrivateKey),
		Organization:   cfg.GhAppOrg,
	}
	client, err := ghclient.New(ghConfig)
	if err != nil {
		log.Fatalf("Failed to create github client: %v", err)
	}
	return client
}

type hookDeps struct {
	watcher       *statusWatcher
	send          func(message string)
	initialWait   time.Duration
	checkInterval time.Duration
	deregTimeout  time.Duration
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// a second signal kills the process while deregistration runs
	context.AfterFunc(ctx, stop)

	info := coreVersion.GetVersionInfo()
	log.Printf("Starting hook application\nVersion: %s\nDate: %s\nCommit SHA: %s\nPlatform: %s\n",
		info.Version, info.BuildDate, info.GitCommit, info.Platform)
	log.Println("Reading configuration")
	loadHookConfiguration()
	readRunnerConfig()

	log.Println("Initializing github client")
	githubClient := initGithubClient()

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

	deps := hookDeps{
		watcher: &statusWatcher{
			api:            githubClient,
			name:           ghaRunnerName,
			backoff:        defaultBackoff,
			offlineLimit:   cfg.RunnerOfflineChecksCount,
			finishedChecks: cfg.RunnerFinishedChecksCount,
		},
		send: func(message string) {
			sendMessageToSqs(message)
			log.Println("Runner sent to sqs")
		},
		initialWait:   15 * time.Second,
		checkInterval: time.Duration(cfg.RunnerStatusCheckPeriod) * time.Second,
		deregTimeout:  time.Duration(cfg.RunnerDeregisterTimeout) * time.Second,
	}
	exit, err := run(ctx, deps, string(message))
	stop()

	switch {
	case errors.Is(err, errRunnerBusy):
		log.Printf("Runner %s is busy, skipped deregistration", ghaRunnerName)
	case err != nil:
		log.Printf("Failed to deregister runner %s: %v", ghaRunnerName, err)
	}
	log.Printf("Hook finished: %s", exit)
	if code := exitCode(exit, err); code != 0 {
		os.Exit(code)
	}
}

func run(ctx context.Context, d hookDeps, message string) (loopExit, error) {
	exit := sendAndWait(ctx, d, message)
	if exit == exitFinished {
		return exit, nil
	}
	log.Printf("Deregistering runner %s: %s", d.watcher.name, exit)
	return exit, deregisterRunner(d.watcher.api, d.watcher.name, d.watcher.backoff, d.deregTimeout)
}

func sendAndWait(ctx context.Context, d hookDeps, message string) loopExit {
	if ctx.Err() != nil {
		log.Println("Terminated before the runner request was sent")
		return exitSignal
	}
	d.send(message)

	log.Println("Waiting for runner to finish")
	select {
	case <-ctx.Done():
		return exitSignal
	case <-time.After(d.initialWait):
	}
	return d.watcher.wait(ctx, d.checkInterval)
}

func exitCode(exit loopExit, deregErr error) int {
	if exit == exitSignal && deregErr != nil && !errors.Is(deregErr, errRunnerBusy) {
		return 1
	}
	return 0
}

func (w *statusWatcher) wait(ctx context.Context, interval time.Duration) loopExit {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return exitSignal
		}
		if exit, done := w.check(ctx); done {
			return exit
		}
		select {
		case <-ctx.Done():
			return exitSignal
		case <-ticker.C:
		}
	}
}

func (w *statusWatcher) check(ctx context.Context) (exit loopExit, done bool) {
	var runners *github.Runners
	lookupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := withRetry(lookupCtx, w.backoff, func() error {
		var err error
		runners, err = w.api.GetRunnerByName(w.name)
		return err
	})
	if err != nil {
		if ctx.Err() != nil {
			return exitSignal, true
		}
		log.Printf("Failed to get runner %s from github after retries: %v", w.name, err)
		return 0, false
	}

	if len(runners.Runners) > 0 {
		if runners.Runners[0].GetStatus() == "online" {
			log.Printf("Runner %s is active", w.name)
			return 0, false
		}
		log.Printf("Runner %s is offline", w.name)
		w.offlineCount++
		if w.offlineCount > w.offlineLimit {
			log.Printf("Runner %s failed to start in secifed interval", w.name)
			return exitOfflineLimit, true
		}
		return 0, false
	}
	if w.checksPassed < w.finishedChecks {
		w.checksPassed++
		log.Printf("Runner %s finished.", w.name)
		return 0, false
	}
	return exitFinished, true
}

var errRunnerBusy = errors.New("runner is busy")

func deregisterRunner(api runnerAPI, name string, backoff backoffConfig, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result := make(chan error, 1)
	go func() { result <- removeIdleRunner(ctx, api, name, backoff) }()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func removeIdleRunner(ctx context.Context, api runnerAPI, name string, backoff backoffConfig) error {
	var runners *github.Runners
	err := withRetry(ctx, backoff, func() error {
		var err error
		runners, err = api.GetRunnerByName(name)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to look up runner %s: %w", name, err)
	}
	if len(runners.Runners) == 0 {
		log.Printf("Runner %s is not registered on github", name)
		return nil
	}

	runner := runners.Runners[0]
	if runner.GetBusy() {
		return errRunnerBusy
	}
	err = withRetry(ctx, backoff, func() error {
		if removeErr := api.RemoveRunner(runner.GetID()); removeErr != nil && !hasStatus(removeErr, http.StatusNotFound) {
			return removeErr
		}
		return nil
	})
	if hasStatus(err, http.StatusUnprocessableEntity) {
		return errRunnerBusy
	}
	if err != nil {
		return fmt.Errorf("failed to remove runner %s: %w", name, err)
	}
	log.Printf("Runner %s removed from github", name)
	return nil
}

func hasStatus(err error, code int) bool {
	var ghErr *github.ErrorResponse
	return errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == code
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

		if !shouldRetry(err) {
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

func shouldRetry(err error) bool {
	if retry, known := classifyGitHubError(err); known {
		return retry
	}
	if isRateLimitError(err) {
		log.Printf("Hit GitHub rate limit, backing off: %v", err)
		return true
	}
	return isRetryableError(err)
}

func classifyGitHubError(err error) (retry, known bool) {
	var rateLimitErr *github.RateLimitError
	var abuseErr *github.AbuseRateLimitError
	if errors.As(err, &rateLimitErr) || errors.As(err, &abuseErr) {
		return true, true
	}
	var respErr *github.ErrorResponse
	if errors.As(err, &respErr) && respErr.Response != nil {
		code := respErr.Response.StatusCode
		return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError, true
	}
	return false, false
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
