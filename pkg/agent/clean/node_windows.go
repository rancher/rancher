package clean

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

const ContainerName = "cattle-node-cleanup"

func Job() error {
	logrus.Infof("Starting clean container job: %s", ContainerName)

	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	containerList, err := c.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, c := range containerList {
		for _, n := range c.Names {
			if n == "/"+ContainerName {
				logrus.Infof("container named %s already exists, exiting.", ContainerName)
				return nil
			}
		}
	}

	binds := []string{
		"\\\\.\\pipe\\docker_engine:\\\\.\\pipe\\docker_engine",
		"c:\\:c:\\host",
		"\\\\.\\pipe\\rancher_wins:\\\\.\\pipe\\rancher_wins",
	}

	container, err := c.ContainerCreate(ctx, &container.Config{
		Image: getAgentImage(),
		Env: []string{
			"AGENT_IMAGE=" + getAgentImage(),
			"PREFIX_PATH=" + os.Getenv("PREFIX_PATH"),
			"WINDOWS_PREFIX_PATH=" + os.Getenv("WINDOWS_PREFIX_PATH"),
		},
		Cmd: []string{"--", "agent", "clean"},
	}, &container.HostConfig{
		Binds: binds,
	}, &network.NetworkingConfig{}, ContainerName)

	if err != nil {
		return err
	}

	return c.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
}

func Node() error {
	logrus.Info("Cleaning up node...")

	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()
	if err := waitForK8sPods(ctx, c); err != nil {
		return fmt.Errorf("error waiting for k8s pods to be removed: %s", err)
	}

	return spawnCleanup()
}

func spawnCleanup() error {
	if err := WriteScript(); err != nil {
		return err
	}
	winsArgs := createWinsArgs("Spawn")
	output, err := exec.Command("wins.exe", winsArgs...).Output()
	if err != nil {
		logrus.Infof(string(output))
		return err
	}
	return nil
}

func WriteScript() error {
	// add a null file to the container for wins to find and make a hash
	psPath := getPowershellPath()
	if !fileExists(psPath) {
		psHostPath := strings.Replace(psPath, "c:\\", "c:\\host\\", 1)

		src, err := os.Open(psHostPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(psPath)
		if err != nil {
			return err
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
	} else {
		logrus.Infof("powershell.exe already exists: %s", psPath)
	}

	// write one to the host for wins cli to call
	scriptBytes := []byte(PowershellScript)
	hostScriptPath := strings.Replace(getScriptPath(), "c:\\", "c:\\host\\", 1)
	if !fileExists(hostScriptPath) {
		logrus.Infof("writing file to host: %s", hostScriptPath)
		if err := ioutil.WriteFile(hostScriptPath, scriptBytes, 0777); err != nil {
			return fmt.Errorf("error writing the cleanup script to the host: %s", err)
		}
	} else {
		logrus.Infof("cleanup script already exists on host: %s", hostScriptPath)
	}

	return nil
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func Links() error {
	if err := WriteScript(); err != nil {
		return err
	}
	winsArgs := createWinsArgs("Network")
	output, err := exec.Command("wins.exe", winsArgs...).Output()
	if err != nil {
		logrus.Infof(string(output))
		return err
	}
	return nil
}

func Paths() error {
	if err := WriteScript(); err != nil {
		return err
	}
	winsArgs := createWinsArgs("Paths")
	output, err := exec.Command("wins.exe", winsArgs...).Output()
	if err != nil {
		logrus.Infof(string(output))
		return err
	}
	return nil
}

func getAgentImage() string {
	agentImage := os.Getenv("AGENT_IMAGE")
	if agentImage == "" {
		agentImage = "rancher/rancher-agent:master"
	}
	return agentImage
}

func cleanDocker() error {
	if err := WriteScript(); err != nil {
		return err
	}
	winsArgs := createWinsArgs("Docker")
	output, err := exec.Command("wins.exe", winsArgs...).Output()
	if err != nil {
		logrus.Infof(string(output))
		return err
	}
	return nil
}

func waitForK8sPods(ctx context.Context, c *client.Client) error {
	// wait for up to 5min for k8s pods to be dropped
	for i := 0; i < 30; i++ {
		logrus.Infof("checking for pods %d out of 30 times", i)
		containerList, err := c.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			return err
		}

		hasPods := false
		for _, c := range containerList {
			for _, n := range c.Names {
				if strings.HasPrefix(n, "/k8s_") {
					hasPods = true
					continue
				}
			}
			if hasPods {
				continue //break out if you already found one
			}
		}

		if hasPods {
			logrus.Info("pods found, waiting 10s and trying again")
			time.Sleep(10 * time.Second)
			continue
		}

		logrus.Info("all pods cleaned, continuing on to more rke cleanup")
		return nil
	}

	return nil
}

func Firewall() error {
	if err := WriteScript(); err != nil {
		return err
	}
	winsArgs := createWinsArgs("Firewall")
	output, err := exec.Command("wins.exe", winsArgs...).Output()
	if err != nil {
		logrus.Infof(string(output))
		return err
	}
	return nil
}

func createWinsArgs(tasks ...string) []string {
	args := fmt.Sprintf("-File %s", getScriptPath())
	if len(tasks) > 0 {
		args = fmt.Sprintf("%s -Tasks %s", args, strings.Join(tasks, ","))
	}

	path := getPowershellPath()
	logrus.Infof("path: %s, args: %s", path, args)

	return []string{
		"cli", "prc", "run",
		"--path", path,
		"--args", args,
	}
}

func spawn(tasks ...string) []string {
	args := fmt.Sprintf("-File %s", getScriptPath())
	if len(tasks) > 0 {
		args = fmt.Sprintf("%s -Tasks %s", args, strings.Join(tasks, ","))
	}

	path := getPowershellPath()
	logrus.Infof("path: %s, args: %s", path, args)

	return []string{
		"cli", "prc", "run",
		"--path", path,
		"--args", args,
	}
}

func getPrefixPath() string {
	prefix := os.Getenv("WINDOWS_PREFIX_PATH")
	if prefix == "" {
		prefix = "c:\\"
	}
	return prefix
}

func getScriptPath() string {
	return filepath.Join(getPrefixPath(), "etc", "rancher", "cleanup.ps1")
}

func getPowershellPath() string {
	return filepath.Join(getPrefixPath(), "etc", "rancher", "powershell.exe")
}
