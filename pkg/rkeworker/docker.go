package rkeworker

import (
	"context"
	"reflect"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type NodeConfig struct {
	APIProxyAddress string                `json:"apiProxyAddress"`
	Certs           string                `json:"certs"`
	Processes       map[string]v3.Process `json:"processes"`
}

func runProcess(ctx context.Context, name string, p v3.Process) error {
	c, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	args := filters.NewArgs()
	args.Add("label", "io.cattle.process.name="+name)

	containers, err := c.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return err
	}

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
		}
	}

	for i := 1; i < len(matchedContainers); i++ {
		if err := remove(ctx, c, matchedContainers[i].ID); err != nil {
			return err
		}
	}

	if len(matchedContainers) > 0 {
		c.ContainerStart(ctx, matchedContainers[0].ID, types.ContainerStartOptions{})
		return nil
	}

	config, hostConfig, _ := services.GetProcessConfig(p)
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels["io.cattle.process.name"] = name
	hostConfig.VolumesFrom = nil

	newContainer, err := c.ContainerCreate(ctx, config, hostConfig, nil, name)
	if err == nil {
		return c.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{})
	}
	return err
}

func remove(ctx context.Context, c *client.Client, id string) error {
	return c.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

func changed(ctx context.Context, c *client.Client, p v3.Process, container types.Container) (bool, error) {
	inspect, err := c.ContainerInspect(ctx, container.ID)
	if err != nil {
		return false, err
	}

	newProcess := v3.Process{
		Command:     inspect.Config.Entrypoint,
		Args:        inspect.Config.Cmd,
		Env:         inspect.Config.Env,
		Image:       inspect.Config.Image,
		VolumesFrom: inspect.HostConfig.VolumesFrom,
		Binds:       inspect.HostConfig.Binds,
		NetworkMode: string(inspect.HostConfig.NetworkMode),
		PidMode:     string(inspect.HostConfig.PidMode),
		Privileged:  inspect.HostConfig.Privileged,
	}

	if len(p.Command) == 0 {
		p.Command = newProcess.Command
	}
	if len(p.Args) == 0 {
		p.Args = newProcess.Args
	}
	if len(p.Env) == 0 {
		p.Env = newProcess.Env
	}
	if p.NetworkMode == "" {
		p.NetworkMode = newProcess.NetworkMode
	}
	if p.PidMode == "" {
		p.PidMode = newProcess.PidMode
	}

	newProcess.HealthCheck.URL = p.HealthCheck.URL
	newProcess.RestartPolicy = p.RestartPolicy

	return !reflect.DeepEqual(newProcess, p), nil
}
