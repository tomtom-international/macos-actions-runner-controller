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

import (
	"encoding/json"
	ghclient "github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/github"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	u "github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"os"
)

func GetGithubClientConfig(c *TarterConfig) (ghConfig ghclient.ClientConfig) {
	smc, err := InitSecretManager(c)
	if err != nil {
		logger.Errorf("Failed to initialize secret manager: %v", err.Error())
		os.Exit(1)
	}

	githubAppSecret, err := smc.GetSecret(c.AwsSecretGitHubApp)
	if err != nil {
		logger.Errorf("Failed to get GitHub App from AWS Secret Manager: %v", err.Error())
		os.Exit(1)
	}
	var ghApp struct {
		AppOrg            u.Int32String `json:"app-organization"`
		AppID             u.Int32String `json:"app-id"`
		AppPrivateKey     u.Int32String `json:"app-private-key"`
		AppInstallationID u.Int32String `json:"app-installation-id"`
	}

	err = json.Unmarshal([]byte(githubAppSecret.String()), &ghApp)
	if err != nil {
		logger.Errorf("Failed to unmarshal GitHub App secret: %v", err.Error())
		os.Exit(1)
	}

	return ghclient.ClientConfig{
		AppID:          int64(ghApp.AppID.IntValue()),
		InstallationID: int64(ghApp.AppInstallationID.IntValue()),
		PrivateKey:     []byte(ghApp.AppPrivateKey.String()),
		Organization:   ghApp.AppOrg.String(),
	}
}
