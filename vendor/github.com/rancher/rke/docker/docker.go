package docker

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/log"
	"github.com/sirupsen/logrus"
)

var K8sDockerVersions = map[string][]string{
	"1.8": {"1.12.6", "1.13.1", "17.03.2"},
}

func DoRunContainer(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig, containerName string, hostname string, plane string) error {
	container, err := dClient.ContainerInspect(ctx, containerName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return err
		}
		if err := UseLocalOrPull(ctx, dClient, hostname, imageCfg.Image, plane); err != nil {
			return err
		}
		resp, err := dClient.ContainerCreate(ctx, imageCfg, hostCfg, nil, containerName)
		if err != nil {
			return fmt.Errorf("Failed to create [%s] container on host [%s]: %v", containerName, hostname, err)
		}
		if err := dClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
		}
		log.Infof(ctx, "[%s] Successfully started [%s] container on host [%s]", plane, containerName, hostname)
		return nil
	}
	// Check for upgrades
	if container.State.Running {
		log.Infof(ctx, "[%s] Container [%s] is already running on host [%s]", plane, containerName, hostname)
		isUpgradable, err := IsContainerUpgradable(ctx, dClient, imageCfg, containerName, hostname, plane)
		if err != nil {
			return err
		}
		if isUpgradable {
			return DoRollingUpdateContainer(ctx, dClient, imageCfg, hostCfg, containerName, hostname, plane)
		}
		return nil
	}

	// start if not running
	log.Infof(ctx, "[%s] Starting stopped container [%s] on host [%s]", plane, containerName, hostname)
	if err := dClient.ContainerStart(ctx, container.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	log.Infof(ctx, "[%s] Successfully started [%s] container on host [%s]", plane, containerName, hostname)
	return nil
}

func DoRollingUpdateContainer(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig, containerName, hostname, plane string) error {
	logrus.Debugf("[%s] Checking for deployed [%s]", plane, containerName)
	isRunning, err := IsContainerRunning(ctx, dClient, hostname, containerName, false)
	if err != nil {
		return err
	}
	if !isRunning {
		log.Infof(ctx, "[%s] Container %s is not running on host [%s]", plane, containerName, hostname)
		return nil
	}
	err = UseLocalOrPull(ctx, dClient, hostname, imageCfg.Image, plane)
	if err != nil {
		return err
	}
	logrus.Debugf("[%s] Stopping old container", plane)
	oldContainerName := "old-" + containerName
	if err := StopRenameContainer(ctx, dClient, hostname, containerName, oldContainerName); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully stopped old container %s on host [%s]", plane, containerName, hostname)
	_, err = CreateContiner(ctx, dClient, hostname, containerName, imageCfg, hostCfg)
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
	log.Infof(ctx, "[remove/%s] Checking if container is running on host [%s]", containerName, hostname)
	// not using the wrapper to check if the error is a NotFound error
	_, err := dClient.ContainerInspect(ctx, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			log.Infof(ctx, "[remove/%s] Container doesn't exist on host [%s]", containerName, hostname)
			return nil
		}
		return err
	}
	log.Infof(ctx, "[remove/%s] Stopping container on host [%s]", containerName, hostname)
	err = StopContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		return err
	}

	log.Infof(ctx, "[remove/%s] Removing container on host [%s]", containerName, hostname)
	err = RemoveContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		return err
	}
	log.Infof(ctx, "[remove/%s] Successfully removed container on host [%s]", containerName, hostname)
	return nil
}

func IsContainerRunning(ctx context.Context, dClient *client.Client, hostname string, containerName string, all bool) (bool, error) {
	logrus.Debugf("Checking if container [%s] is running on host [%s]", containerName, hostname)
	containers, err := dClient.ContainerList(ctx, types.ContainerListOptions{All: all})
	if err != nil {
		return false, fmt.Errorf("Can't get Docker containers for host [%s]: %v", hostname, err)

	}
	for _, container := range containers {
		if container.Names[0] == "/"+containerName {
			return true, nil
		}
	}
	return false, nil
}

func localImageExists(ctx context.Context, dClient *client.Client, hostname string, containerImage string) (bool, error) {
	logrus.Debugf("Checking if image [%s] exists on host [%s]", containerImage, hostname)
	_, _, err := dClient.ImageInspectWithRaw(ctx, containerImage)
	if err != nil {
		if client.IsErrNotFound(err) {
			logrus.Debugf("Image [%s] does not exist on host [%s]: %v", containerImage, hostname, err)
			return false, nil
		}
		return false, fmt.Errorf("Error checking if image [%s] exists on host [%s]: %v", containerImage, hostname, err)
	}
	logrus.Debugf("Image [%s] exists on host [%s]", containerImage, hostname)
	return true, nil
}

func pullImage(ctx context.Context, dClient *client.Client, hostname string, containerImage string) error {
	out, err := dClient.ImagePull(ctx, containerImage, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("Can't pull Docker image [%s] for host [%s]: %v", containerImage, hostname, err)
	}
	defer out.Close()
	if logrus.GetLevel() == logrus.DebugLevel {
		io.Copy(os.Stdout, out)
	} else {
		io.Copy(ioutil.Discard, out)
	}

	return nil
}

func UseLocalOrPull(ctx context.Context, dClient *client.Client, hostname string, containerImage string, plane string) error {
	log.Infof(ctx, "[%s] Checking image [%s] on host [%s]", plane, containerImage, hostname)
	imageExists, err := localImageExists(ctx, dClient, hostname, containerImage)
	if err != nil {
		return err
	}
	if imageExists {
		log.Infof(ctx, "[%s] No pull necessary, image [%s] exists on host [%s]", plane, containerImage, hostname)
		return nil
	}
	log.Infof(ctx, "[%s] Pulling image [%s] on host [%s]", plane, containerImage, hostname)
	if err := pullImage(ctx, dClient, hostname, containerImage); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully pulled image [%s] on host [%s]", plane, containerImage, hostname)
	return nil
}

func RemoveContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) error {
	err := dClient.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Can't remove Docker container [%s] for host [%s]: %v", containerName, hostname, err)
	}
	return nil
}

func StopContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) error {
	err := dClient.ContainerStop(ctx, containerName, nil)
	if err != nil {
		return fmt.Errorf("Can't stop Docker container [%s] for host [%s]: %v", containerName, hostname, err)
	}
	return nil
}

func RenameContainer(ctx context.Context, dClient *client.Client, hostname string, oldContainerName string, newContainerName string) error {
	err := dClient.ContainerRename(ctx, oldContainerName, newContainerName)
	if err != nil {
		return fmt.Errorf("Can't rename Docker container [%s] for host [%s]: %v", oldContainerName, hostname, err)
	}
	return nil
}

func StartContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) error {
	if err := dClient.ContainerStart(ctx, containerName, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	return nil
}

func CreateContiner(ctx context.Context, dClient *client.Client, hostname string, containerName string, imageCfg *container.Config, hostCfg *container.HostConfig) (container.ContainerCreateCreatedBody, error) {
	created, err := dClient.ContainerCreate(ctx, imageCfg, hostCfg, nil, containerName)
	if err != nil {
		return container.ContainerCreateCreatedBody{}, fmt.Errorf("Failed to create [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	return created, nil
}

func InspectContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) (types.ContainerJSON, error) {
	inspection, err := dClient.ContainerInspect(ctx, containerName)
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("Failed to inspect [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	return inspection, nil
}

func StopRenameContainer(ctx context.Context, dClient *client.Client, hostname string, oldContainerName string, newContainerName string) error {
	if err := StopContainer(ctx, dClient, hostname, oldContainerName); err != nil {
		return err
	}
	if err := WaitForContainer(ctx, dClient, oldContainerName); err != nil {
		return nil
	}
	err := RenameContainer(ctx, dClient, hostname, oldContainerName, newContainerName)
	return err
}

func WaitForContainer(ctx context.Context, dClient *client.Client, containerName string) error {
	statusCh, errCh := dClient.ContainerWait(ctx, containerName, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("Error wating for container [%s]: %v", containerName, err)
		}
	case <-statusCh:
	}
	return nil
}

func IsContainerUpgradable(ctx context.Context, dClient *client.Client, imageCfg *container.Config, containerName string, hostname string, plane string) (bool, error) {
	logrus.Debugf("[%s] Checking if container [%s] is eligible for upgrade on host [%s]", plane, containerName, hostname)
	// this should be moved to a higher layer.

	containerInspect, err := InspectContainer(ctx, dClient, hostname, containerName)
	if err != nil {
		return false, err
	}
	if containerInspect.Config.Image != imageCfg.Image || !reflect.DeepEqual(containerInspect.Config.Cmd, imageCfg.Cmd) {
		logrus.Debugf("[%s] Container [%s] is eligible for updgrade on host [%s]", plane, containerName, hostname)
		return true, nil
	}
	logrus.Debugf("[%s] Container [%s] is not eligible for updgrade on host [%s]", plane, containerName, hostname)
	return false, nil
}

func IsSupportedDockerVersion(info types.Info, K8sVersion string) (bool, error) {
	// Docker versions are not semver compliant since stable/edge version (17.03 and higher) so we need to check if the reported ServerVersion starts with a compatible version
	for _, DockerVersion := range K8sDockerVersions[K8sVersion] {
		DockerVersionRegexp := regexp.MustCompile("^" + DockerVersion)
		if DockerVersionRegexp.MatchString(info.ServerVersion) {
			return true, nil
		}

	}
	return false, nil
}

func ReadFileFromContainer(ctx context.Context, dClient *client.Client, hostname, container, filePath string) (string, error) {
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

func ReadContainerLogs(ctx context.Context, dClient *client.Client, containerName string) (io.ReadCloser, error) {
	return dClient.ContainerLogs(ctx, containerName, types.ContainerLogsOptions{ShowStdout: true})

}
