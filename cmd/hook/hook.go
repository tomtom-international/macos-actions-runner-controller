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
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/sqs"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	coreVersion "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version"
	"gopkg.in/yaml.v3"
)

const (
	runnerStatusCheckPeriod   = 60 * time.Second
	runnerFinishedChecksCount = 1
)

var (
	cfg          hookConfig
	runnerConfig types.RunnerConfig
	githubClient *ghclient.Client

	checksPassed = 0
)

type hookConfig struct {
	AwsRegion           string `env:"AWS_REGION" envDefault:"eu-west-1"`
	AwsRunnerSQSUrl     string `env:"AWS_RUNNER_SQS_URL"`
	GhaJITConfig        string `env:"ACTIONS_RUNNER_INPUT_JITCONFIG"`
	RunnerConfigFile    string `env:"RUNNER_CONFIG_FILE" envDefault:"/opt/tarter-hook/runner_config.yaml"`
	GhaRunnerGroup      string `env:"GHA_RUNNER_GROUP"`
	GhAppPrivateKey     string `env:"GH_APP_PRIVATE_KEY"`
	GhAppOrg            string `env:"GH_APP_ORG"`
	GhAppID             int    `env:"GH_APP_ID"`
	GhAppInstallationID int    `env:"GH_APP_INSTALLATION_ID"`
}

func loadHookConfiguration() {
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("error: failed to load configuration. %s", err.Error())
	}
}

func readRunnerConfig() {
	data, err := os.ReadFile(cfg.RunnerConfigFile)
	if err != nil {
		log.Fatalf("error: failed to read runner configuration. %s", err.Error())
	}
	err = yaml.Unmarshal(data, &runnerConfig)
	if err != nil {
		log.Fatalf("error: failed to unmarshal runner configuration. %s", err.Error())
	}
}

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
		log.Fatalf("Failed to create github client: %s", err.Error())
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
	ticker := time.NewTicker(runnerStatusCheckPeriod)
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

func checkStatus(ghaRunnerName string) bool {
	runners, err := githubClient.GetRunnerByName(ghaRunnerName)

	if err != nil {
		log.Printf("Failed to get runner <%v> from github: %v", ghaRunnerName, err)
		return true
	}

	if len(runners.Runners) > 0 {
		log.Printf("Runner <%v> is active", ghaRunnerName)
		return true
	}
	if checksPassed < runnerFinishedChecksCount {
		checksPassed++
		log.Printf("Runner <%v> finished.", ghaRunnerName)
		return true
	}
	log.Printf("Runner <%v> not registered on github", ghaRunnerName)
	return false
}

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
