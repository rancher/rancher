package rkeworker

import (
	"context"
	"io"
	"reflect"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	RKEContainerNameLabel  = "io.rancher.rke.container.name"
	CattleProcessNameLabel = "io.cattle.process.name"
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
		return nil
	}

	config, hostConfig, _ := services.GetProcessConfig(p)
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[RKEContainerNameLabel] = name

	newContainer, err := c.ContainerCreate(ctx, config, hostConfig, nil, name)
	if client.IsErrImageNotFound(err) {
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

func changed(ctx context.Context, c *client.Client, expectedProcess v3.Process, container types.Container) (bool, error) {
	actualDockerInspect, err := c.ContainerInspect(ctx, container.ID)
	if err != nil {
		return false, err
	}
	defaultDockerInspect, _, err := c.ImageInspectWithRaw(ctx, actualDockerInspect.Image)
	if err != nil {
		return false, err
	}
	actualProcess := v3.Process{
		Command:     actualDockerInspect.Config.Entrypoint,
		Args:        actualDockerInspect.Config.Cmd,
		Env:         actualDockerInspect.Config.Env,
		Image:       actualDockerInspect.Config.Image,
		Binds:       actualDockerInspect.HostConfig.Binds,
		NetworkMode: string(actualDockerInspect.HostConfig.NetworkMode),
		PidMode:     string(actualDockerInspect.HostConfig.PidMode),
		Privileged:  actualDockerInspect.HostConfig.Privileged,
		VolumesFrom: actualDockerInspect.HostConfig.VolumesFrom,
		Labels:      actualDockerInspect.Config.Labels,
	}

	if len(expectedProcess.Command) == 0 {
		expectedProcess.Command = actualProcess.Command
	}
	if len(expectedProcess.Args) == 0 {
		expectedProcess.Args = actualProcess.Args
	}
	if len(expectedProcess.Env) == 0 {
		expectedProcess.Env = actualProcess.Env
	}
	if expectedProcess.NetworkMode == "" {
		expectedProcess.NetworkMode = actualProcess.NetworkMode
	}
	if expectedProcess.PidMode == "" {
		expectedProcess.PidMode = actualProcess.PidMode
	}
	if len(expectedProcess.Labels) == 0 {
		expectedProcess.Labels = actualProcess.Labels
	}

	// Don't detect changes on these fields
	actualProcess.Name = expectedProcess.Name
	actualProcess.HealthCheck.URL = expectedProcess.HealthCheck.URL
	actualProcess.RestartPolicy = expectedProcess.RestartPolicy

	changed := false
	t := reflect.TypeOf(actualProcess)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "Command" {
			leftMap := sliceToMap(expectedProcess.Command)
			rightMap := sliceToMap(actualProcess.Command)

			if reflect.DeepEqual(leftMap, rightMap) {
				continue
			}
		} else if f.Name == "Env" {
			expectedEnvs := make(map[string]string, 8)
			for _, env := range defaultDockerInspect.Config.Env {
				es := strings.SplitN(env, "=", 2)
				expectedEnvs[es[0]] = es[1]
			}
			for _, env := range expectedProcess.Env {
				es := strings.SplitN(env, "=", 2)
				expectedEnvs[es[0]] = es[1]
			}

			actualEnvs := make(map[string]string, 8)
			for _, env := range actualProcess.Env {
				es := strings.SplitN(env, "=", 2)
				actualEnvs[es[0]] = es[1]
			}

			if reflect.DeepEqual(expectedEnvs, actualEnvs) {
				continue
			}
		} else if f.Name == "Labels" {
			processLabels := make(map[string]string)
			for k, v := range defaultDockerInspect.Config.Labels {
				processLabels[k] = v
			}
			for k, v := range expectedProcess.Labels {
				processLabels[k] = v
			}

			if reflect.DeepEqual(processLabels, actualProcess.Labels) {
				continue
			}
		} else if f.Name == "Publish" {
			expectedExposedPortSet, expectedBindings, err := nat.ParsePortSpecs(reflect.ValueOf(expectedProcess).Field(i).Interface().([]string))
			if err != nil {
				return false, err
			}
			expectedExposedPorts := natPortSetToSlice(expectedExposedPortSet)
			nat.SortPortMap(expectedExposedPorts, expectedBindings)

			actualExposedPortSet := make(map[nat.Port]struct{}, 0)
			actualBindings := actualDockerInspect.HostConfig.PortBindings
			for port := range actualBindings {
				if _, exists := actualExposedPortSet[port]; !exists {
					actualExposedPortSet[port] = struct{}{}
				}
			}
			actualExposedPorts := natPortSetToSlice(actualExposedPortSet)
			nat.SortPortMap(actualExposedPorts, actualBindings)

			if reflect.DeepEqual(actualBindings, nat.PortMap(expectedBindings)) {
				continue
			}
		}

		left := reflect.ValueOf(actualProcess).Field(i).Interface()
		right := reflect.ValueOf(expectedProcess).Field(i).Interface()
		if !reflect.DeepEqual(left, right) {
			logrus.Infof("For process %s, %s has changed from %v to %v", expectedProcess.Name, f.Name, right, left)
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

func natPortSetToSlice(args map[nat.Port]struct{}) []nat.Port {
	result := make([]nat.Port, 0, len(args))
	for arg := range args {
		result = append(result, arg)
	}
	return result
}
