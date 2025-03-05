/*
 * Copyright 2025 TomTom N.V.
 * Copyright 2019 The Kubernetes Authors.
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

package version

import (
	"fmt"
	"runtime"
)

// Default values contains versioning information.
var (
	Version   = "v0.0.0"
	GitCommit = ""                     // sha1 from git, output of $(git rev-parse HEAD)
	BuildDate = "1970-01-01T00:00:00Z" // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
)

// VersionInfo contains versioning information.
type VersionInfo struct {
	Version   string `json:"major"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	Platform  string `json:"platform"`
}

func GetVersionInfo() VersionInfo {
	// These variables come from -ldflags settings. If empty, they will be set to the default values.
	return VersionInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
