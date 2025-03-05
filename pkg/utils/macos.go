package utils

import (
	"os/exec"
	"strings"
)

func GetMacOSVersion() (string, error) {
	out, err := exec.Command("sw_vers", "-productVersion").CombinedOutput()
	if err != nil {
		return "", err
	}
	version := strings.TrimRight(string(out), "\r\n")
	return version, nil
}
