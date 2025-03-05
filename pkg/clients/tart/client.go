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

package tart

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

const (
	runnerNameFile              = "runner_name"
	runnerGroupFile             = "runner_group"
	runnerHostsFile             = "runner_hosts"
	runnerLabelsFile            = "runner_labels"
	runnerRegistrationTokenFile = ".runner_register_key"
	runnerConfigFile            = "runner_config"
	runnerHostname              = "runner_hostname"
	runnerJitConfigFile         = "runner_jit_config"
)

// TODO: Migrate ghaRunners to start with runnerConfig
//type runnerConfig struct {
//	Name              string   `json:"name" yaml:"name"`
//	RunnerGroup       string   `json:"runnerGroup" yaml:"runnerGroup"`
//	Labels            []string `json:"labels" yaml:"labels"`
//	RegistrationToken string   `json:"registrationToken" yaml:"registrationToken"`
//}

type Client struct {
	tartPath     string
	configFolder string
}

func NewClient(tartPath string, configFolder string) *Client {
	return &Client{
		tartPath:     tartPath,
		configFolder: configFolder,
	}
}

func (c *Client) GetTartVersion() (string, error) {
	out, err := exec.Command(c.tartPath, "--version").CombinedOutput()
	if err != nil {
		return "", err
	}
	version := strings.TrimRight(string(out), "\r\n")
	return version, nil
}

func (c *Client) BuildCMDArguments(tartVMName string, config types.RunnerConfig) []string {
	var args []string

	args = append(args, "run")
	if config.NoGraphics {
		args = append(args, "--no-graphics")
	}

	if config.SoftnetNetwork.Enable {
		args = append(args, "--net-softnet")
	}

	if config.SoftnetNetwork.Enable && len(config.SoftnetNetwork.AllowedCIDRs) > 0 {
		args = append(args, fmt.Sprintf("--net-softnet-allow=%v", strings.Join(config.SoftnetNetwork.AllowedCIDRs[:], ",")))
	}

	if config.CacheVolumePath != "" {
		args = append(args, fmt.Sprintf("--dir=%v:ro,tag=runnerCache", config.CacheVolumePath))
	}

	if !config.DisableRootDiskOptions {
		args = append(args, fmt.Sprintf("--root-disk-opts=sync=none,caching=cached"))
	}
	args = append(args, fmt.Sprintf("--dir=config:%v:ro", path.Join(c.configFolder, tartVMName)))
	args = append(args, tartVMName)

	return args
}

func (c *Client) BuildCommand(stdout *bytes.Buffer, stderr *bytes.Buffer, args ...string) *exec.Cmd {
	cmd := exec.Command(c.tartPath, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}

func (c *Client) SetupRunnerConfiguration(
	nodeName string,
	runnerId string,
	tartVMName string,
	config types.RunnerConfig,
	registrationToken string) (string, error) {

	runnerConfigFolder := path.Join(c.configFolder, tartVMName)
	err := os.MkdirAll(runnerConfigFolder, 0777)
	if err != nil {
		return "", fmt.Errorf("failed to create runner config folder: %v", err)
	}
	err = c.cloneRunnerImage(tartVMName, config.BaseImage)
	if err != nil {
		return "", err
	}
	err = c.setRunnerResources(tartVMName, config.Cpu, config.Memory)
	if err != nil {
		return "", err
	}
	err = c.setRunnerHosts(runnerConfigFolder, config.RunnerHosts)
	if err != nil {
		return "", fmt.Errorf("failed to set runner hosts: %v", err)
	}
	err = c.setRunnerHostname(runnerConfigFolder, config.RunnerHostname, runnerId, config.ReleaseVersion)
	if err != nil {
		return "", fmt.Errorf("failed to set runner hostname: %v", err)
	}

	// Temporary solution until we have JIT config.
	var ghaRunnerName string

	if config.JitConfig != "" {
		err = c.SetJITConfig(runnerConfigFolder, config.JitConfig)
		if err != nil {
			return "", fmt.Errorf("failed to set JIT config: %v", err)
		}
	} else {
		err = c.setRunnerGroup(runnerConfigFolder, config.RunnerGroup)
		if err != nil {
			return "", fmt.Errorf("failed to set runner group: %v", err)
		}
		err = c.setRunnerLabels(runnerConfigFolder, config.RunnerLabels)
		if err != nil {
			return "", fmt.Errorf("failed to set runner labels: %v", err)
		}
		err = c.setRegistrationToken(runnerConfigFolder, registrationToken)
		if err != nil {
			return "", fmt.Errorf("failed to set runner github registration token: %v", err)
		}
		ghaRunnerName, err = c.setGhaRunnerName(nodeName, runnerConfigFolder, runnerId, config.ReleaseVersion)
		if err != nil {
			return "", fmt.Errorf("failed to set github action runner name: %v", err)
		}
	}
	return ghaRunnerName, nil
}

func (c *Client) CleanupRunnerConfiguration(tartVMName string) {
	err := os.RemoveAll(path.Join(c.configFolder, tartVMName))
	if err != nil {
		logger.Debugf("Unable to remove Tart config: %s", err.Error())
		return
	}
	c.removeRunnerImage(tartVMName)
}

func (c *Client) SetJITConfig(runnerConfigFolder string, jitConfig string) error {
	cfg, err := base64.StdEncoding.DecodeString(jitConfig)
	if err != nil {
		return fmt.Errorf("failed to decode JIT config: %v", err)
	}
	err = writeToFile(path.Join(runnerConfigFolder, runnerJitConfigFile), string(cfg))
	if err != nil {
		return fmt.Errorf("failed to write JIT config: %v", err)
	}
	return nil
}

func (c *Client) removeRunnerImage(tartVMName string) {
	var stderrBuf bytes.Buffer
	cmd := exec.Command(c.tartPath, "delete", tartVMName)
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err != nil {
		logger.Debugf("Unable to remove Tart image: %v", stderrBuf.String())
	}
}

func (c *Client) cloneRunnerImage(tartVMName string, baseImage string) error {
	var stderrBuf bytes.Buffer
	cmd := exec.Command(c.tartPath, "clone", baseImage, tartVMName)
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to clone Tart image: %v", stderrBuf.String())
	}
	return nil
}

func (c *Client) setRunnerResources(tartVMName string, cpuCount utils.Int32String, memory utils.Int32String) error {
	var stderrBuf bytes.Buffer
	cmd := exec.Command(c.tartPath, "set", tartVMName,
		"--cpu", cpuCount.String(),
		"--memory", memory.String())

	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set runner resources: %v", stderrBuf.String())
	}
	return nil
}

func (c *Client) setRunnerGroup(runnerConfigFolder string, runnerGroup string) error {
	return writeToFile(path.Join(runnerConfigFolder, runnerGroupFile), runnerGroup)
}

func (c *Client) setRunnerHosts(runnerConfigFolder string, runnerHosts []types.RunnerHosts) error {
	var hosts string
	for _, host := range runnerHosts {
		for _, hostname := range host.Hostnames {
			hosts += fmt.Sprintf("%v %v\n", host.IP, hostname)
		}
	}
	if hosts != "" {
		return writeToFile(path.Join(runnerConfigFolder, runnerHostsFile), hosts)
	}
	return nil
}

func (c *Client) setRunnerHostname(runnerConfigFolder string, hostname string, runnerId string, runnerVersion string) error {
	if hostname == "" {
		timestamp := time.Now().Unix()
		version := strings.ReplaceAll(runnerVersion, ".", "-")
		hostname = fmt.Sprintf("runner-%v-%v-v%v.local", runnerId, timestamp, version)
	}
	return writeToFile(path.Join(runnerConfigFolder, runnerHostname), hostname)
}

func (c *Client) setRunnerLabels(runnerConfigFolder string, runnerLabels []string) error {
	var labels string
	for _, label := range runnerLabels {
		labels += fmt.Sprintf("%v\n", label)
	}
	if labels != "" {
		return writeToFile(path.Join(runnerConfigFolder, runnerLabelsFile), labels)
	}
	return nil
}

func (c *Client) setRegistrationToken(runnerConfigFolder string, token string) error {
	return writeToFile(path.Join(runnerConfigFolder, runnerRegistrationTokenFile), token)
}

func (c *Client) setGhaRunnerName(nodeName string, runnerConfigFolder string, runnerId string, runnerVersion string) (string, error) {
	timestamp := time.Now().Unix()
	if strings.IndexByte(nodeName, '.') != -1 {
		nodeName = nodeName[:strings.IndexByte(nodeName, '.')]
	}

	ghaRunnerName := fmt.Sprintf("%v-%v-%v-v%v", nodeName, runnerId, timestamp, runnerVersion)

	err := writeToFile(path.Join(runnerConfigFolder, runnerNameFile), ghaRunnerName)
	if err != nil {
		return "", err
	}
	return ghaRunnerName, nil
}

func writeToFile(filePath string, data string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	_, err = f.WriteString(data)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)
	return nil
}
