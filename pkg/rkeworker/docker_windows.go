package rkeworker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

// Windows process is different with Linux as below:
// 1. Deletes all matching containers
// 2. Doesn't support log linker
func runProcess(ctx context.Context, name string, p v3.Process, _, _ bool) error {
	c, err := client.NewClientWithOpts(client.FromEnv)
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
	for _, container := range containers {
		// remove all matching containers
		err := remove(ctx, c, container.ID)
		if err != nil {
			return err
		}
	}

	config, hostConfig, _ := services.GetProcessConfig(p)
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[RKEContainerNameLabel] = name

	// create new container
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
	if err != nil {
		return err
	}

	// wait for running container to exit
	waitStatusChan := waitExit(ctx, c, newContainer.ID)
	if err := c.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	status := <-waitStatusChan
	if err := status.error(); err != nil {
		return errors.Wrapf(err, "error waiting for %s container", name)
	}

	return nil
}

func waitExit(rootCtx context.Context, dockerCli *client.Client, containerID string) <-chan waitStatus {
	resultC, errC := dockerCli.ContainerWait(rootCtx, containerID, container.WaitConditionNextExit)

	statusC := make(chan waitStatus)
	go func(ctx context.Context) {
		select {
		case result := <-resultC:
			if result.Error != nil {
				statusC <- waitStatus{
					code:  125,
					cause: fmt.Errorf(result.Error.Message),
				}
			} else {
				statusC <- waitStatus{
					code: int(result.StatusCode),
				}
			}
		case err := <-errC:
			statusC <- waitStatus{
				code:  125,
				cause: err,
			}
		}
	}(rootCtx)

	return statusC
}

type waitStatus struct {
	code  int
	cause error
}

func (s waitStatus) error() error {
	if s.code != 0 {
		if s.cause != nil {
			return s.cause
		}

		return fmt.Errorf("exit code %d", s.code)
	}

	return nil
}
