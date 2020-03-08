package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	ref "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/metadata"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	DockerRegistryURL = "docker.io"
	// RestartTimeout in seconds
	RestartTimeout = 5
	// StopTimeout in seconds
	StopTimeout = 5
	// RetryCount is the amount of retries for Docker operations
	RetryCount = 3
)

type dockerConfig struct {
	Auths map[string]authConfig `json:"auths,omitempty"`
}

type authConfig types.AuthConfig

func DoRunContainer(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig,
	containerName string, hostname string, plane string, prsMap map[string]v3.PrivateRegistry) error {
	if dClient == nil {
		return fmt.Errorf("[%s] Failed to run container: docker client is nil for container [%s] on host [%s]",
			plane, containerName, hostname)
	}
	container, err := InspectContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return err
		}
		if err := UseLocalOrPull(ctx, dClient, hostname, imageCfg.Image, plane, prsMap); err != nil {
			return fmt.Errorf("Failed to pull image [%s] on host [%s]: %v", imageCfg.Image, hostname, err)
		}
		_, err := CreateContainer(ctx, dClient, hostname, containerName, imageCfg, hostCfg)
		if err != nil {
			return fmt.Errorf("Failed to create [%s] container on host [%s]: %v", containerName, hostname, err)
		}
		if err := StartContainer(ctx, dClient, hostname, containerName); err != nil {
			return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
		}
		log.Infof(ctx, "[%s] Successfully started [%s] container on host [%s]", plane, containerName, hostname)
		return nil
	}
	// Check for upgrades
	if container.State.Running {
		// check if container is in a restarting loop
		if container.State.Restarting {
			logrus.Debugf("[%s] Container [%s] is in a restarting loop [%s]", plane, containerName, hostname)
			err = RestartContainer(ctx, dClient, hostname, containerName)
			if err != nil {
				return err
			}
		}
		logrus.Debugf("[%s] Container [%s] is already running on host [%s]", plane, containerName, hostname)
		isUpgradable, err := IsContainerUpgradable(ctx, dClient, imageCfg, hostCfg, containerName, hostname, plane)
		if err != nil {
			return err
		}
		if isUpgradable {
			return DoRollingUpdateContainer(ctx, dClient, imageCfg, hostCfg, containerName, hostname, plane, prsMap)
		}
		return nil
	}

	// start if not running
	logrus.Infof("[%s] Starting stopped container [%s] on host [%s]", plane, containerName, hostname)
	if err := StartContainer(ctx, dClient, hostname, containerName); err != nil {
		return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	log.Infof(ctx, "[%s] Successfully started [%s] container on host [%s]", plane, containerName, hostname)
	return nil
}

func DoRunOnetimeContainer(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig, containerName string, hostname string, plane string, prsMap map[string]v3.PrivateRegistry) error {
	if dClient == nil {
		return fmt.Errorf("[%s] Failed to run container: docker client is nil for container [%s] on host [%s]", plane, containerName, hostname)
	}
	_, err := InspectContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return err
		}
		if err := UseLocalOrPull(ctx, dClient, hostname, imageCfg.Image, plane, prsMap); err != nil {
			return err
		}
		_, err := CreateContainer(ctx, dClient, hostname, containerName, imageCfg, hostCfg)
		if err != nil {
			return fmt.Errorf("Failed to create [%s] container on host [%s]: %v", containerName, hostname, err)
		}
		if err := StartContainer(ctx, dClient, hostname, containerName); err != nil {
			return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
		}
		log.Infof(ctx, "Successfully started [%s] container on host [%s]", containerName, hostname)
		log.Infof(ctx, "Waiting for [%s] container to exit on host [%s]", containerName, hostname)
		exitCode, err := WaitForContainer(ctx, dClient, hostname, containerName)
		if err != nil {
			return fmt.Errorf("Container [%s] did not complete in time on host [%s]", containerName, hostname)
		}
		if exitCode != 0 {
			stderr, stdout, err := GetContainerLogsStdoutStderr(ctx, dClient, containerName, "1", false)
			if err != nil {
				return fmt.Errorf("Unable to retrieve logs for container [%s] on host [%s]: %v", containerName, hostname, err)
			}
			stderr = strings.TrimSuffix(stderr, "\n")
			stdout = strings.TrimSuffix(stdout, "\n")
			return fmt.Errorf("Container [%s] exited with non-zero exit code [%d] on host [%s]: stdout: %s, stderr: %s", containerName, exitCode, hostname, stdout, stderr)
		}
		return nil
	}
	return err
}

func DoRollingUpdateContainer(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig, containerName, hostname, plane string, prsMap map[string]v3.PrivateRegistry) error {
	if dClient == nil {
		return fmt.Errorf("[%s] Failed rolling update of container: docker client is nil for container [%s] on host [%s]", plane, containerName, hostname)
	}
	logrus.Debugf("[%s] Checking for deployed [%s]", plane, containerName)
	isRunning, err := IsContainerRunning(ctx, dClient, hostname, containerName, false)
	if err != nil {
		return err
	}
	if !isRunning {
		logrus.Debugf("[%s] Container %s is not running on host [%s]", plane, containerName, hostname)
		return nil
	}
	err = UseLocalOrPull(ctx, dClient, hostname, imageCfg.Image, plane, prsMap)
	if err != nil {
		return err
	}
	logrus.Debugf("[%s] Stopping old container", plane)
	oldContainerName := "old-" + containerName
	if err := StopRenameContainer(ctx, dClient, hostname, containerName, oldContainerName); err != nil {
		return err
	}
	logrus.Debugf("[%s] Successfully stopped old container %s on host [%s]", plane, containerName, hostname)
	_, err = CreateContainer(ctx, dClient, hostname, containerName, imageCfg, hostCfg)
	if err != nil {
		return fmt.Errorf("Failed to create [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	if err := StartContainer(ctx, dClient, hostname, containerName); err != nil {
		return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	log.Infof(ctx, "[%s] Successfully updated [%s] container on host [%s]", plane, containerName, hostname)
	logrus.Debugf("[%s] Removing old container", plane)
	err = RemoveContainer(ctx, dClient, hostname, oldContainerName)
	return err
}

func DoRemoveContainer(ctx context.Context, dClient *client.Client, containerName, hostname string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to remove container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	logrus.Debugf("[remove/%s] Checking if container is running on host [%s]", containerName, hostname)
	_, err := InspectContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			logrus.Debugf("[remove/%s] Container doesn't exist on host [%s]", containerName, hostname)
			return nil
		}
		return err
	}
	logrus.Debugf("[remove/%s] Removing container on host [%s]", containerName, hostname)
	err = RemoveContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		return err
	}
	log.Infof(ctx, "[remove/%s] Successfully removed container on host [%s]", containerName, hostname)
	return nil
}

func IsContainerRunning(ctx context.Context, dClient *client.Client, hostname string, containerName string, all bool) (bool, error) {
	if dClient == nil {
		return false, fmt.Errorf("Failed to check if container is running: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	var containers []types.Container
	var err error
	for i := 1; i <= RetryCount; i++ {
		logrus.Infof("Checking if container [%s] is running on host [%s], try #%d", containerName, hostname, i)
		containers, err = dClient.ContainerList(ctx, types.ContainerListOptions{All: all})
		if err != nil {
			logrus.Warnf("Error checking if container [%s] is running on host [%s]: %v", containerName, hostname, err)
			continue
		}
		break
	}
	if err != nil {
		return false, fmt.Errorf("Error checking if container [%s] is running on host [%s]: %v", containerName, hostname, err)
	}
	for _, container := range containers {
		if len(container.Names) != 0 && container.Names[0] == "/"+containerName {
			return true, nil
		}
	}
	return false, nil
}

func localImageExists(ctx context.Context, dClient *client.Client, hostname string, containerImage string) error {
	var err error
	for i := 1; i <= RetryCount; i++ {
		logrus.Debugf("Checking if image [%s] exists on host [%s], try #%d", containerImage, hostname, i)
		_, _, err = dClient.ImageInspectWithRaw(ctx, containerImage)
		if err != nil {
			if client.IsErrNotFound(err) {
				logrus.Debugf("Image [%s] does not exist on host [%s]: %v", containerImage, hostname, err)
				return err
			}
			logrus.Debugf("Error checking if image [%s] exists on host [%s]: %v", containerImage, hostname, err)
			continue
		}
		logrus.Infof("Image [%s] exists on host [%s]", containerImage, hostname)
		return nil
	}
	return fmt.Errorf("Error checking if image [%s] exists on host [%s]: %v", containerImage, hostname, err)
}

func pullImage(ctx context.Context, dClient *client.Client, hostname string, containerImage string,
	prsMap map[string]v3.PrivateRegistry) error {
	var out io.ReadCloser
	var err error
	pullOptions := types.ImagePullOptions{}

	regAuth, prURL, err := GetImageRegistryConfig(containerImage, prsMap)
	if err != nil {
		return err
	}
	if regAuth != "" && prURL == DockerRegistryURL {
		pullOptions.PrivilegeFunc = tryRegistryAuth(prsMap[prURL])
	}
	pullOptions.RegistryAuth = regAuth

	// Retry up to RetryCount times to pull image
	for i := 1; i <= RetryCount; i++ {
		logrus.Infof("Pulling image [%s] on host [%s], try #%d", containerImage, hostname, i)
		out, err = dClient.ImagePull(ctx, containerImage, pullOptions)
		if err != nil {
			logrus.Warnf("Can't pull Docker image [%s] on host [%s]: %v", containerImage, hostname, err)
			continue
		}
		defer out.Close()
		if logrus.GetLevel() == logrus.TraceLevel {
			io.Copy(os.Stdout, out)
		} else {
			io.Copy(ioutil.Discard, out)
		}
		return nil
	}
	// If the for loop does not return, return the error
	return err
}

func UseLocalOrPull(ctx context.Context, dClient *client.Client, hostname string, containerImage string, plane string,
	prsMap map[string]v3.PrivateRegistry) error {
	if dClient == nil {
		return fmt.Errorf("[%s] Failed to use local image or pull: docker client is nil for container [%s] on host [%s]", plane, containerImage, hostname)
	}
	var err error
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		// Increasing wait time on retry, but not on the first two try
		if i > 2 {
			time.Sleep(time.Duration(i) * time.Second)
		}

		if err = localImageExists(ctx, dClient, hostname, containerImage); err == nil {
			// Return if image exists to prevent pulling
			return nil
		}

		// If error, log and retry
		if !client.IsErrNotFound(err) {
			logrus.Debugf("[%s] %v", plane, err)
			continue
		}

		// Try pulling when not found and if error, log and retry
		err = pullImage(ctx, dClient, hostname, containerImage, prsMap)
		if err != nil {
			logrus.Debugf("[%s] Can't pull Docker image [%s] on host [%s]: %v", plane, containerImage, hostname, err)
			continue
		}
	}
	// If the for loop does not return, return the error
	if err != nil {
		// Although error should be logged in the caller stack, logging the final error here just in case. Mostly
		// because error logging was reduced in other places
		logrus.Warnf("[%s] Can't pull Docker image [%s] on host [%s]: %v", plane, containerImage, hostname, err)
	}
	return err
}

func RemoveContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to remove container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	var err error
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		logrus.Infof("Removing container [%s] on host [%s], try #%d", containerName, hostname, i)
		err = dClient.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		if err != nil {
			logrus.Warningf("Can't remove Docker container [%s] for host [%s]: %v", containerName, hostname, err)
			continue
		}
		return nil
	}
	return err
}

func RestartContainer(ctx context.Context, dClient *client.Client, hostname, containerName string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to restart container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	var err error
	restartTimeout := RestartTimeout * time.Second
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		logrus.Infof("Restarting container [%s] on host [%s], try #%d", containerName, hostname, i)
		err = dClient.ContainerRestart(ctx, containerName, &restartTimeout)
		if err != nil {
			logrus.Warningf("Can't restart Docker container [%s] for host [%s]: %v", containerName, hostname, err)
			continue
		}
		return nil
	}
	return err
}
func StopContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to stop container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	var err error
	// define the stop timeout
	stopTimeoutDuration := StopTimeout * time.Second
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		logrus.Infof("Stopping container [%s] on host [%s] with stopTimeoutDuration [%s], try #%d", containerName, hostname, stopTimeoutDuration, i)
		err = dClient.ContainerStop(ctx, containerName, &stopTimeoutDuration)
		if err != nil {
			logrus.Warningf("Can't stop Docker container [%s] for host [%s]: %v", containerName, hostname, err)
			continue
		}
		return nil
	}
	return err
}

func RenameContainer(ctx context.Context, dClient *client.Client, hostname string, oldContainerName string, newContainerName string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to rename container: docker client is nil for container [%s] on host [%s]", oldContainerName, hostname)
	}
	var err error
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		logrus.Infof("Renaming container [%s] to [%s] on host [%s], try #%d", oldContainerName, newContainerName, hostname, i)
		err = dClient.ContainerRename(ctx, oldContainerName, newContainerName)
		if err != nil {
			logrus.Warningf("Can't rename Docker container [%s] to [%s] for host [%s]: %v", oldContainerName, newContainerName, hostname, err)
			continue
		}
		return nil
	}
	return err
}

func StartContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to start container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	var err error
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		logrus.Infof("Starting container [%s] on host [%s], try #%d", containerName, hostname, i)
		err = dClient.ContainerStart(ctx, containerName, types.ContainerStartOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "bind: address already in use") {
				return err
			}
			logrus.Warningf("Can't start Docker container [%s] on host [%s]: %v", containerName, hostname, err)
			continue
		}
		return nil
	}
	return err
}

func CreateContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string, imageCfg *container.Config, hostCfg *container.HostConfig) (container.ContainerCreateCreatedBody, error) {
	if dClient == nil {
		return container.ContainerCreateCreatedBody{}, fmt.Errorf("Failed to create container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	var created container.ContainerCreateCreatedBody
	var err error
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		created, err = dClient.ContainerCreate(ctx, imageCfg, hostCfg, nil, containerName)
		if err != nil {
			logrus.Warningf("Failed to create Docker container [%s] on host [%s]: %v", containerName, hostname, err)
			continue
		}
		return created, nil
	}
	return container.ContainerCreateCreatedBody{}, fmt.Errorf("Failed to create Docker container [%s] on host [%s]: %v", containerName, hostname, err)
}

func InspectContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) (types.ContainerJSON, error) {
	if dClient == nil {
		return types.ContainerJSON{}, fmt.Errorf("Failed to inspect container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	var inspection types.ContainerJSON
	var err error
	// Retry up to RetryCount times to see if image exists
	for i := 1; i <= RetryCount; i++ {
		inspection, err = dClient.ContainerInspect(ctx, containerName)
		if err != nil {
			if client.IsErrNotFound(err) {
				return types.ContainerJSON{}, err
			}
			logrus.Warningf("Failed to inspect Docker container [%s] on host [%s]: %v", containerName, hostname, err)
			continue
		}
		return inspection, nil
	}
	return types.ContainerJSON{}, fmt.Errorf("Failed to inspect Docker container [%s] on host [%s]: %v", containerName, hostname, err)
}

func StopRenameContainer(ctx context.Context, dClient *client.Client, hostname string, oldContainerName string, newContainerName string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to stop and rename container: docker client is nil for container [%s] on host [%s]", oldContainerName, hostname)
	}
	// make sure we don't have an old old-container from a previous broken update
	exists, err := IsContainerRunning(ctx, dClient, hostname, newContainerName, true)
	if err != nil {
		return err
	}
	if exists {
		if err := RemoveContainer(ctx, dClient, hostname, newContainerName); err != nil {
			return err
		}
	}
	if err := StopContainer(ctx, dClient, hostname, oldContainerName); err != nil {
		return err
	}
	if _, err := WaitForContainer(ctx, dClient, hostname, oldContainerName); err != nil {
		return err
	}
	return RenameContainer(ctx, dClient, hostname, oldContainerName, newContainerName)

}

func WaitForContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) (int64, error) {
	if dClient == nil {
		return 1, fmt.Errorf("Failed waiting for container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	// 5 minutes timeout, especially for transferring snapshots
	for retries := 0; retries < 300; retries++ {
		log.Infof(ctx, "Waiting for [%s] container to exit on host [%s]", containerName, hostname)
		container, err := InspectContainer(ctx, dClient, hostname, containerName)
		if err != nil {
			return 1, fmt.Errorf("Could not inspect container [%s] on host [%s]: %s", containerName, hostname, err)
		}
		if container.State.Running {
			log.Infof(ctx, "Container [%s] is still running on host [%s]", containerName, hostname)
			time.Sleep(1 * time.Second)
			continue
		}
		logrus.Debugf("Exit code for [%s] container on host [%s] is [%d]", containerName, hostname, int64(container.State.ExitCode))
		return int64(container.State.ExitCode), nil
	}
	return 1, fmt.Errorf("Container [%s] did not exit in time on host [%s]", containerName, hostname)
}

func IsContainerUpgradable(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig, containerName string, hostname string, plane string) (bool, error) {
	if dClient == nil {
		return false, fmt.Errorf("[%s] Failed checking if container is upgradable: docker client is nil for container [%s] on host [%s]", plane, containerName, hostname)
	}
	logrus.Debugf("[%s] Checking if container [%s] is eligible for upgrade on host [%s]", plane, containerName, hostname)
	// this should be moved to a higher layer.

	containerInspect, err := InspectContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		return false, err
	}
	// image inspect to compare the env correctly
	imageInspect, _, err := dClient.ImageInspectWithRaw(ctx, imageCfg.Image)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return false, err
		}
		logrus.Debugf("[%s] Container [%s] is eligible for upgrade on host [%s]", plane, containerName, hostname)
		return true, nil
	}
	if containerInspect.Config.Image != imageCfg.Image ||
		!sliceEqualsIgnoreOrder(containerInspect.Config.Entrypoint, imageCfg.Entrypoint) ||
		!sliceEqualsIgnoreOrder(containerInspect.Config.Cmd, imageCfg.Cmd) ||
		!isContainerEnvChanged(containerInspect.Config.Env, imageCfg.Env, imageInspect.Config.Env) ||
		!sliceEqualsIgnoreOrder(containerInspect.HostConfig.Binds, hostCfg.Binds) ||
		!securityOptsliceEqualsIgnoreOrder(containerInspect.HostConfig.SecurityOpt, hostCfg.SecurityOpt) {
		logrus.Debugf("[%s] Container [%s] is eligible for upgrade on host [%s]", plane, containerName, hostname)
		return true, nil
	}
	logrus.Debugf("[%s] Container [%s] is not eligible for upgrade on host [%s]", plane, containerName, hostname)
	return false, nil
}

func sliceEqualsIgnoreOrder(left, right []string) bool {
	if equal := sets.NewString(left...).Equal(sets.NewString(right...)); !equal {
		logrus.Debugf("slice is not equal, showing data in new value which is not in old value: %v", sets.NewString(right...).Difference(sets.NewString(left...)))
		logrus.Debugf("slice is not equal, showing data in old value which is not in new value: %v", sets.NewString(left...).Difference(sets.NewString(right...)))
		return false
	}
	return true
}

func securityOptsliceEqualsIgnoreOrder(left, right []string) bool {
	if equal := sets.NewString(left...).Equal(sets.NewString(right...)); !equal {
		logrus.Debugf("slice is not equal, showing data in new value which is not in old value: %v", sets.NewString(right...).Difference(sets.NewString(left...)))
		diff := sets.NewString(left...).Difference(sets.NewString(right...))
		logrus.Debugf("slice is not equal, showing data in old value which is not in new value: %v", diff)
		// Docker sets label=disable automatically on all non labeled containers with will result in a false diff between spec and the actual running container
		// If the diff matches the disable label exactly, we still report true as being equal
		if equal := sets.NewString([]string{"label=disable"}...).Equal(diff); equal {
			logrus.Debugf("returning equal as true because diff matches the automatically added disable label for SELinux which can be ignored: %v", diff)
			return true
		}
		return false
	}
	return true
}

func IsSupportedDockerVersion(info types.Info, K8sVersion string) (bool, error) {
	dockerVersion, err := semver.NewVersion(info.ServerVersion)
	if err != nil {
		return false, err
	}
	for _, DockerVersion := range metadata.K8sVersionToDockerVersions[K8sVersion] {
		supportedDockerVersion, err := convertToSemver(DockerVersion)
		if err != nil {
			return false, err
		}
		if dockerVersion.Major == supportedDockerVersion.Major && dockerVersion.Minor == supportedDockerVersion.Minor {
			return true, nil
		}

	}
	return false, nil
}

func ReadFileFromContainer(ctx context.Context, dClient *client.Client, hostname, container, filePath string) (string, error) {
	if dClient == nil {
		return "", fmt.Errorf("Failed reading file from container: docker client is nil for container [%s] on host [%s]", container, hostname)
	}
	reader, _, err := dClient.CopyFromContainer(ctx, container, filePath)
	if err != nil {
		return "", fmt.Errorf("Failed to copy file [%s] from container [%s] on host [%s]: %v", filePath, container, hostname, err)
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)
	if _, err := tarReader.Next(); err != nil {
		return "", err
	}
	file, err := ioutil.ReadAll(tarReader)
	if err != nil {
		return "", err
	}
	return string(file), nil
}

func ReadContainerLogs(ctx context.Context, dClient *client.Client, containerName string, follow bool, tail string) (io.ReadCloser, error) {
	if dClient == nil {
		return nil, fmt.Errorf("Failed reading container logs: docker client is nil for container [%s]", containerName)
	}
	var logs io.ReadCloser
	var err error
	for i := 1; i <= RetryCount; i++ {
		logs, err = dClient.ContainerLogs(ctx, containerName, types.ContainerLogsOptions{Follow: follow, ShowStdout: true, ShowStderr: true, Timestamps: false, Tail: tail})
		if err != nil {
			logrus.Warnf("Can't read container logs for container [%s]: %v", containerName, err)
			continue
		}
		return logs, nil
	}
	return nil, err
}

func GetContainerLogsStdoutStderr(ctx context.Context, dClient *client.Client, containerName, tail string, follow bool) (string, string, error) {
	if dClient == nil {
		return "", "", fmt.Errorf("Failed to get container logs stdout and stderr: docker client is nil for container [%s]", containerName)
	}
	var containerStderr bytes.Buffer
	var containerStdout bytes.Buffer
	var containerErrLog, containerStdLog string
	clogs, logserr := ReadContainerLogs(ctx, dClient, containerName, follow, tail)
	if logserr != nil || clogs == nil {
		logrus.Debugf("logserr: %v", logserr)
		return containerErrLog, containerStdLog, fmt.Errorf("Failed to get gather logs from container [%s]: %v", containerName, logserr)
	}
	defer clogs.Close()
	stdcopy.StdCopy(&containerStdout, &containerStderr, clogs)
	containerErrLog = containerStderr.String()
	containerStdLog = containerStdout.String()
	return containerErrLog, containerStdLog, nil
}

func tryRegistryAuth(pr v3.PrivateRegistry) types.RequestPrivilegeFunc {
	return func() (string, error) {
		return getRegistryAuth(pr)
	}
}

func getRegistryAuth(pr v3.PrivateRegistry) (string, error) {
	authConfig := types.AuthConfig{
		Username: pr.User,
		Password: pr.Password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}

func GetImageRegistryConfig(image string, prsMap map[string]v3.PrivateRegistry) (string, string, error) {
	/*
		Image can be passed as
		- Example1: repo.com/foo/bar/rancher/rke-tools:v0.1.51
		or
		- Example2: repo.com/rancher/rke-tools:v0.1.51 // image2
		or
		- rancher/rke-tools
		Where the repo can be:
		- repo.com
		or
		- repo.com/foo/bar
		When checking for the repo presence in prsMap, the following repo will be found:
		- Example1: repo.com/foo/bar
		- Exmaple2: repo.com
	*/
	namedImage, err := ref.ParseNormalizedNamed(image)
	if err != nil {
		return "", "", err
	}
	if len(prsMap) == 0 {
		return "", "", nil
	}
	regURL := ref.Domain(namedImage)
	regPath := ref.Path(namedImage)

	splitPath := strings.Split(regPath, "/")
	if len(splitPath) > 2 {
		splitPath = splitPath[:len(splitPath)-2]
		regPath = strings.Join(splitPath, "/")
		regURL = fmt.Sprintf("%s/%s", regURL, regPath)
	}

	if pr, ok := prsMap[regURL]; ok {
		logrus.Debugf("Found regURL %v", regURL)
		// We do this if we have some docker.io login information
		regAuth, err := getRegistryAuth(pr)
		return regAuth, pr.URL, err
	}

	return "", "", nil
}

func convertToSemver(version string) (*semver.Version, error) {
	compVersion := strings.SplitN(version, ".", 3)
	if len(compVersion) != 3 {
		return nil, fmt.Errorf("The default version is not correct")
	}
	compVersion[2] = "0"
	return semver.NewVersion(strings.Join(compVersion, "."))
}

func isContainerEnvChanged(containerEnv, imageConfigEnv, dockerfileEnv []string) bool {
	// remove PATH env from the container env
	allImageEnv := append(imageConfigEnv, dockerfileEnv...)
	return sliceEqualsIgnoreOrder(allImageEnv, containerEnv)
}

func GetKubeletDockerConfig(prsMap map[string]v3.PrivateRegistry) (string, error) {
	auths := map[string]authConfig{}

	for url, pr := range prsMap {
		auth := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", pr.User, pr.Password)))
		auths[url] = authConfig{Auth: auth}
	}
	cfg, err := json.Marshal(dockerConfig{auths})
	if err != nil {
		return "", err
	}
	return string(cfg), nil
}

func DoRestartContainer(ctx context.Context, dClient *client.Client, containerName, hostname string) error {
	if dClient == nil {
		return fmt.Errorf("Failed to restart container: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	logrus.Debugf("[restart/%s] Checking if container is running on host [%s]", containerName, hostname)
	_, err := InspectContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			logrus.Debugf("[restart/%s] Container doesn't exist on host [%s]", containerName, hostname)
			return nil
		}
		return err
	}
	logrus.Debugf("[restart/%s] Restarting container on host [%s]", containerName, hostname)
	err = RestartContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		return err
	}
	log.Infof(ctx, "[restart/%s] Successfully restarted container on host [%s]", containerName, hostname)
	return nil
}

func GetContainerOutput(ctx context.Context, dClient *client.Client, containerName, hostname string) (int64, string, string, error) {
	if dClient == nil {
		return 1, "", "", fmt.Errorf("Failed to get container output: docker client is nil for container [%s] on host [%s]", containerName, hostname)
	}
	status, err := WaitForContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		return 1, "", "", err
	}

	stderr, stdout, err := GetContainerLogsStdoutStderr(ctx, dClient, containerName, "1", false)
	if err != nil {
		return 1, "", "", err
	}

	return status, stdout, stderr, nil
}
