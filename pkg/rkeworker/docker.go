package rkeworker

import (
	"context"
	"io"
	"os"
	"reflect"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type NodeConfig struct {
	ClusterName string                `json:"clusterName"`
	Certs       string                `json:"certs"`
	Processes   map[string]v3.Process `json:"processes"`
	Files       []v3.File             `json:"files"`
}

func runProcess(ctx context.Context, name string, p v3.Process, start bool) error {
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

	newContainer, err := c.ContainerCreate(ctx, config, hostConfig, nil, name)
	if client.IsErrImageNotFound(err) {
		var output io.ReadCloser
		imagePullOptions := types.ImagePullOptions{}
		if p.ImageRegistryAuthConfig != "" {
			imagePullOptions.RegistryAuth = p.ImageRegistryAuthConfig
			imagePullOptions.PrivilegeFunc = func() (string, error) { return p.ImageRegistryAuthConfig, nil }
		}
		output, err = c.ImagePull(ctx, config.Image, types.ImagePullOptions{})
		if err != nil {
			return err
		}
		defer output.Close()
		io.Copy(os.Stdout, output)
		newContainer, err = c.ContainerCreate(ctx, config, hostConfig, nil, name)
	}
	if err == nil && start {
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
		Binds:       inspect.HostConfig.Binds,
		NetworkMode: string(inspect.HostConfig.NetworkMode),
		PidMode:     string(inspect.HostConfig.PidMode),
		Privileged:  inspect.HostConfig.Privileged,
		VolumesFrom: inspect.HostConfig.VolumesFrom,
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

	// Don't detect changes on these fields
	newProcess.Name = p.Name
	newProcess.HealthCheck.URL = p.HealthCheck.URL
	newProcess.RestartPolicy = p.RestartPolicy

	changed := false
	t := reflect.TypeOf(newProcess)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "Command" {
			leftMap := sliceToMap(p.Command)
			rightMap := sliceToMap(newProcess.Command)
			if reflect.DeepEqual(leftMap, rightMap) {
				continue
			}
		} else if f.Name == "Env" {
			changed := false
			for _, v := range p.Env {
				if !slice.ContainsString(newProcess.Env, v) {
					changed = true
					break
				}
			}

			if !changed {
				continue
			}
		}

		left := reflect.ValueOf(newProcess).Field(i).Interface()
		right := reflect.ValueOf(p).Field(i).Interface()
		if !reflect.DeepEqual(left, right) {
			logrus.Infof("For process %s, %s has changed from %v to %v", p.Name, f.Name, right, left)
			changed = true
		}
	}

	return changed, nil
}

func sliceToMap(args []string) map[string]bool {
	result := map[string]bool{}
	for _, arg := range args {
		result[arg] = true
	}
	return result
}
