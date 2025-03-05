package utils

import (
	"fmt"
	"os"
	"strings"
)

func GetHostname() (string, error) {
	hostName, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	hostName = strings.TrimSpace(hostName)
	if len(hostName) == 0 {
		return "", fmt.Errorf("empty hostname")
	}

	return strings.ToLower(hostName), nil
}
