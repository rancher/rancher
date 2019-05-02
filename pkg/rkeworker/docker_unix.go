// +build !windows

package rkeworker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func runProcess(ctx context.Context, name string, p v3.Process, start, forceRestart bool) error {
	c, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	defer c.Close()

	args := filters.NewArgs()
	args.Add("label", RKEContainerNameLabel+"="+name)
	// to handle upgrades of old container
	oldArgs := filters.NewArgs()
	oldArgs.Add("label", CattleProcessNameLabel+"="+name)

	containers, err := c.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return err
	}
	oldContainers, err := c.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: oldArgs,
	})
	if err != nil {
		return err
	}
	containers = append(containers, oldContainers...)
	var matchedContainers []types.Container
	for _, container := range containers {
		changed, err := changed(ctx, c, p, container)
		if err != nil {
			return err
		}

		if changed {
			err := remove(ctx, c, container.ID)
			if err != nil {
				return err
			}
		} else {
			matchedContainers = append(matchedContainers, container)
			if forceRestart {
				if err := restart(ctx, c, container.ID); err != nil {
					return err
				}
			}
		}
	}

	for i := 1; i < len(matchedContainers); i++ {
		if err := remove(ctx, c, matchedContainers[i].ID); err != nil {
			return err
		}
	}

	if len(matchedContainers) > 0 {
		if strings.Contains(name, "share-mnt") {
			inspect, err := c.ContainerInspect(ctx, matchedContainers[0].ID)
			if err != nil {
				return err
			}
			if inspect.State != nil && inspect.State.Status == "exited" && inspect.State.ExitCode == 0 {
				return nil
			}
		}
		c.ContainerStart(ctx, matchedContainers[0].ID, types.ContainerStartOptions{})
		if !strings.Contains(name, "share-mnt") {
			runLogLinker(ctx, c, name, p)
		}
		return nil
	}

	config, hostConfig, _ := services.GetProcessConfig(p)
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[RKEContainerNameLabel] = name

	newContainer, err := c.ContainerCreate(ctx, config, hostConfig, nil, name)
	if client.IsErrNotFound(err) {
		var output io.ReadCloser
		imagePullOptions := types.ImagePullOptions{}
		if p.ImageRegistryAuthConfig != "" {
			imagePullOptions.RegistryAuth = p.ImageRegistryAuthConfig
			imagePullOptions.PrivilegeFunc = func() (string, error) { return p.ImageRegistryAuthConfig, nil }
		}
		output, err = c.ImagePull(ctx, config.Image, imagePullOptions)
		if err != nil {
			return err
		}
		defer output.Close()
		io.Copy(logrus.StandardLogger().Writer(), output)
		newContainer, err = c.ContainerCreate(ctx, config, hostConfig, nil, name)
	}
	if err == nil && start {
		if err := c.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{}); err != nil {
			return err
		}
		if !strings.Contains(name, "share-mnt") {
			return runLogLinker(ctx, c, name, p)
		}
		return nil
	}
	return err
}

func runLogLinker(ctx context.Context, c *client.Client, containerName string, p v3.Process) error {
	inspect, err := c.ContainerInspect(ctx, containerName)
	if err != nil {
		return err
	}
	containerID := inspect.ID
	containerLogPath := inspect.LogPath
	containerLogLink := fmt.Sprintf("%s/%s_%s.log", hosts.RKELogsPath, containerName, containerID)
	logLinkerName := fmt.Sprintf("%s-%s", services.LogLinkContainerName, containerName)
	config := &container.Config{
		Image: p.Image,
		Tty:   true,
		Entrypoint: []string{
			"sh",
			"-c",
			fmt.Sprintf("mkdir -p %s ; ln -s %s %s", hosts.RKELogsPath, containerLogPath, containerLogLink),
		},
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			"/var/lib:/var/lib",
		},
		Privileged: true,
	}
	// remove log linker if its already exists
	remove(ctx, c, logLinkerName)

	newContainer, err := c.ContainerCreate(ctx, config, hostConfig, nil, logLinkerName)
	if err != nil {
		return err
	}
	if err := c.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	// remove log linker after start
	return remove(ctx, c, logLinkerName)
}
