package docker

import (
	"archive/tar"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/coreos/go-semver/semver"
	ref "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	DockerRegistryURL = "docker.io"
)

var K8sDockerVersions = map[string][]string{
	"1.8":  {"1.11.x", "1.12.x", "1.13.x", "17.03.x"},
	"1.9":  {"1.11.x", "1.12.x", "1.13.x", "17.03.x"},
	"1.10": {"1.11.x", "1.12.x", "1.13.x", "17.03.x"},
}

func DoRunContainer(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig, containerName string, hostname string, plane string, prsMap map[string]v3.PrivateRegistry) error {
	container, err := dClient.ContainerInspect(ctx, containerName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return err
		}
		if err := UseLocalOrPull(ctx, dClient, hostname, imageCfg.Image, plane, prsMap); err != nil {
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
		logrus.Debugf("[%s] Container [%s] is already running on host [%s]", plane, containerName, hostname)
		isUpgradable, err := IsContainerUpgradable(ctx, dClient, imageCfg, containerName, hostname, plane)
		if err != nil {
			return err
		}
		if isUpgradable {
			return DoRollingUpdateContainer(ctx, dClient, imageCfg, hostCfg, containerName, hostname, plane, prsMap)
		}
		return nil
	}

	// start if not running
	logrus.Debugf("[%s] Starting stopped container [%s] on host [%s]", plane, containerName, hostname)
	if err := dClient.ContainerStart(ctx, container.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, hostname, err)
	}
	log.Infof(ctx, "[%s] Successfully started [%s] container on host [%s]", plane, containerName, hostname)
	return nil
}

func DoRollingUpdateContainer(ctx context.Context, dClient *client.Client, imageCfg *container.Config, hostCfg *container.HostConfig, containerName, hostname, plane string, prsMap map[string]v3.PrivateRegistry) error {
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
	logrus.Debugf("[remove/%s] Checking if container is running on host [%s]", containerName, hostname)
	// not using the wrapper to check if the error is a NotFound error
	_, err := dClient.ContainerInspect(ctx, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			logrus.Debugf("[remove/%s] Container doesn't exist on host [%s]", containerName, hostname)
			return nil
		}
		return err
	}
	logrus.Debugf("[remove/%s] Stopping container on host [%s]", containerName, hostname)
	err = StopContainer(ctx, dClient, hostname, containerName)
	if err != nil {
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

func pullImage(ctx context.Context, dClient *client.Client, hostname string, containerImage string, prsMap map[string]v3.PrivateRegistry) error {
	pullOptions := types.ImagePullOptions{}

	regAuth, prURL, err := GetImageRegistryConfig(containerImage, prsMap)
	if err != nil {
		return err
	}
	if regAuth != "" && prURL == DockerRegistryURL {
		pullOptions.PrivilegeFunc = tryRegistryAuth(prsMap[prURL])
	}
	pullOptions.RegistryAuth = regAuth

	out, err := dClient.ImagePull(ctx, containerImage, pullOptions)
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

func UseLocalOrPull(ctx context.Context, dClient *client.Client, hostname string, containerImage string, plane string, prsMap map[string]v3.PrivateRegistry) error {
	logrus.Debugf("[%s] Checking image [%s] on host [%s]", plane, containerImage, hostname)
	imageExists, err := localImageExists(ctx, dClient, hostname, containerImage)
	if err != nil {
		return err
	}
	if imageExists {
		logrus.Debugf("[%s] No pull necessary, image [%s] exists on host [%s]", plane, containerImage, hostname)
		return nil
	}
	log.Infof(ctx, "[%s] Pulling image [%s] on host [%s]", plane, containerImage, hostname)
	if err := pullImage(ctx, dClient, hostname, containerImage, prsMap); err != nil {
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
	if err := WaitForContainer(ctx, dClient, hostname, oldContainerName); err != nil {
		return nil
	}
	err := RenameContainer(ctx, dClient, hostname, oldContainerName, newContainerName)
	return err
}

func WaitForContainer(ctx context.Context, dClient *client.Client, hostname string, containerName string) error {
	statusCh, errCh := dClient.ContainerWait(ctx, containerName, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("Error waiting for container [%s] on host [%s]: %v", containerName, hostname, err)
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
	if containerInspect.Config.Image != imageCfg.Image ||
		!sliceEqualsIgnoreOrder(containerInspect.Config.Entrypoint, imageCfg.Entrypoint) ||
		!sliceEqualsIgnoreOrder(containerInspect.Config.Cmd, imageCfg.Cmd) {
		logrus.Debugf("[%s] Container [%s] is eligible for updgrade on host [%s]", plane, containerName, hostname)
		return true, nil
	}
	logrus.Debugf("[%s] Container [%s] is not eligible for updgrade on host [%s]", plane, containerName, hostname)
	return false, nil
}

func sliceEqualsIgnoreOrder(left, right []string) bool {
	return sets.NewString(left...).Equal(sets.NewString(right...))
}

func IsSupportedDockerVersion(info types.Info, K8sVersion string) (bool, error) {
	dockerVersion, err := semver.NewVersion(info.ServerVersion)
	if err != nil {
		return false, err
	}
	for _, DockerVersion := range K8sDockerVersions[K8sVersion] {
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
	return dClient.ContainerLogs(ctx, containerName, types.ContainerLogsOptions{Follow: true, ShowStdout: true, ShowStderr: true, Timestamps: false})
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
	namedImage, err := ref.ParseNormalizedNamed(image)
	if err != nil {
		return "", "", err
	}
	regURL := ref.Domain(namedImage)
	if pr, ok := prsMap[regURL]; ok {
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
