package rkeworker

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/services"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	RKEContainerNameLabel  = "io.rancher.rke.container.name"
	CattleProcessNameLabel = "io.cattle.process.name"
	ShareMntContainerName  = "share-mnt"
)

type NodeConfig struct {
	ClusterName  string                `json:"clusterName"`
	Certs        string                `json:"certs"`
	Processes    map[string]v3.Process `json:"processes"`
	Files        []v3.File             `json:"files"`
	UpgradeToken string                `json:"upgradeToken"`
}

func runProcess(ctx context.Context, name string, p v3.Process, start, forceRestart bool) error {
	c, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	defer c.Close()

	args := filters.NewArgs()
	args.Add("label", RKEContainerNameLabel+"="+name)
	// to handle upgrades of containers created in v2.0.x
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
		inspect, err := c.ContainerInspect(ctx, matchedContainers[0].ID)
		if err != nil {
			return err
		}

		// share-mnt does not need to be in running state/does not have to be restarted if it ran successfully
		if strings.Contains(name, ShareMntContainerName) {
			if inspect.State != nil && inspect.State.Status == "exited" && inspect.State.ExitCode == 0 {
				return nil
			}
		}
		// ignore service-sidekick if it is present (other containers just use the volumes)
		if strings.Contains(name, services.SidekickContainerName) {
			return nil
		}

		// if container is running, no need to start and run log linker
		if inspect.State != nil && inspect.State.Status == "running" {
			return nil
		}

		c.ContainerStart(ctx, matchedContainers[0].ID, types.ContainerStartOptions{})
		// Both ShareMntContainerName & services.SidekickContainerName are caught before here, we just never need to run it for those containers
		if !strings.Contains(name, ShareMntContainerName) && !strings.Contains(name, services.SidekickContainerName) {
			runLogLinker(ctx, c, name, p)
		}
		return nil
	}

	// Host is used to determine if selinux is enabled in the Docker daemon, but this is not needed for workers as the components sharing files in service-sidekick all run as privileged
	emptyHost := hosts.Host{}
	config, hostConfig, _ := services.GetProcessConfig(p, &emptyHost)
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
		if !strings.Contains(name, ShareMntContainerName) {
			return runLogLinker(ctx, c, name, p)
		}
		return nil
	}
	return err
}

func remove(ctx context.Context, c *client.Client, id string) error {
	return c.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

func restart(ctx context.Context, c *client.Client, id string) error {
	timeoutDuration := 10 * time.Second
	return c.ContainerRestart(ctx, id, &timeoutDuration)
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
	actualProcess.ImageRegistryAuthConfig = expectedProcess.ImageRegistryAuthConfig

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
				if len(es) == 2 {
					expectedEnvs[es[0]] = es[1]
				}
			}
			for _, env := range expectedProcess.Env {
				es := strings.SplitN(env, "=", 2)
				if len(es) == 2 {
					expectedEnvs[es[0]] = es[1]
				} else {
					expectedEnvs[es[0]] = "_host_related_env_"
				}
			}

			actualEnvs := make(map[string]string, 8)
			for _, env := range actualProcess.Env {
				es := strings.SplitN(env, "=", 2)
				if len(es) == 2 {
					actualEnvs[es[0]] = es[1]
				}
			}

			isNothingChange := true
			for expectedEnvName, expectedEnvVal := range expectedEnvs {
				if expectedEnvVal == "_host_related_env_" {
					continue
				}

				if expectedEnvVal != actualEnvs[expectedEnvName] {
					isNothingChange = false
					break
				}
			}
			if isNothingChange {
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
			logrus.Infof("For process %s, %s has changed from %v to %v", expectedProcess.Name, f.Name, left, right)
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
		Entrypoint: []string{
			"sh",
			"-c",
			fmt.Sprintf("mkdir -p %s ; ln -s %s %s", hosts.RKELogsPath, containerLogPath, containerLogLink),
		},
	}
	if runtime.GOOS == "windows" { // compatible with Windows
		config = &container.Config{
			Image: p.Image,
			Entrypoint: []string{
				"pwsh", "-NoLogo", "-NonInteractive", "-Command",
				fmt.Sprintf(`& {$d="%s"; $t="%s"; $p="%s"; if (-not (Test-Path -PathType Container -Path $d)) {New-Item -ItemType Directory -Path $d -ErrorAction Ignore | Out-Null;} if (-not (Test-Path -PathType Leaf -Path $p)) {New-Item -ItemType SymbolicLink -Target $t -Path $p | Out-Null;}}`,
					filepath.Join("c:/", hosts.RKELogsPath),
					containerLogPath,
					filepath.Join("c:/", containerLogLink),
				),
			},
		}
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			"/var/lib:/var/lib",
		},
		Privileged:  true,
		NetworkMode: "none",
	}
	if runtime.GOOS == "windows" { // compatible with Windows
		hostConfig = &container.HostConfig{
			Binds: []string{
				// symbolic link source: docker container logs location
				"c:/ProgramData:c:/ProgramData",
				// symbolic link target
				"c:/var/lib:c:/var/lib",
			},
			NetworkMode: "none",
		}
	}
	// remove log linker if its already exists
	remove(ctx, c, logLinkerName)

	/*
		the following logic is as same as `docker run --rm`
	*/
	newContainer, err := c.ContainerCreate(ctx, config, hostConfig, nil, logLinkerName)
	if err != nil {
		return err
	}
	statusC := waitContainerExit(ctx, c, newContainer.ID)
	if err := c.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	status := <-statusC
	if err := status.error(); err != nil {
		return err
	}
	return remove(ctx, c, logLinkerName)
}

func waitContainerExit(rootCtx context.Context, dockerCli *client.Client, containerID string) <-chan containerWaitingStatus {
	resultC, errC := dockerCli.ContainerWait(rootCtx, containerID, container.WaitConditionNextExit)

	statusC := make(chan containerWaitingStatus)
	go func(ctx context.Context) {
		select {
		case result := <-resultC:
			if result.Error != nil {
				statusC <- containerWaitingStatus{
					code:  125,
					cause: fmt.Errorf(result.Error.Message),
				}
			} else {
				statusC <- containerWaitingStatus{
					code: int(result.StatusCode),
				}
			}
		case err := <-errC:
			statusC <- containerWaitingStatus{
				code:  125,
				cause: err,
			}
		}
	}(rootCtx)

	return statusC
}

type containerWaitingStatus struct {
	code  int
	cause error
}

func (s containerWaitingStatus) error() error {
	if s.code != 0 {
		if s.cause != nil {
			return s.cause
		}

		return fmt.Errorf("exit code %d", s.code)
	}

	return nil
}
