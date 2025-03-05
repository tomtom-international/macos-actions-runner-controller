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
