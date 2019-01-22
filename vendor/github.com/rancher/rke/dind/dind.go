package dind

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/util"
	"github.com/sirupsen/logrus"
)

const (
	DINDImage           = "docker:17.03-dind"
	DINDContainerPrefix = "rke-dind"
	DINDPlane           = "dind"
	DINDNetwork         = "dind-network"
	DINDSubnet          = "172.18.0.0/16"
)

func StartUpDindContainer(ctx context.Context, dindAddress, dindNetwork, dindStorageDriver, dindDNS string) (string, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}
	// its recommended to use host's storage driver
	dockerInfo, err := cli.Info(ctx)
	if err != nil {
		return "", err
	}
	storageDriver := dindStorageDriver
	if len(storageDriver) == 0 {
		storageDriver = dockerInfo.Driver
	}

	// Get dind container name
	containerName := fmt.Sprintf("%s-%s", DINDContainerPrefix, dindAddress)
	_, err = cli.ContainerInspect(ctx, containerName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return "", err
		}
		if err := docker.UseLocalOrPull(ctx, cli, cli.DaemonHost(), DINDImage, DINDPlane, nil); err != nil {
			return "", err
		}
		binds := []string{
			fmt.Sprintf("/var/lib/kubelet-%s:/var/lib/kubelet:shared", containerName),
		}
		isLink, err := util.IsSymlink("/etc/resolv.conf")
		if err != nil {
			return "", err
		}
		if isLink {
			logrus.Infof("[%s] symlinked [/etc/resolv.conf] file detected. Using [%s] as DNS server.", DINDPlane, dindDNS)
		} else {
			binds = append(binds, "/etc/resolv.conf:/etc/resolv.conf")
		}
		imageCfg := &container.Config{
			Image: DINDImage,
			Entrypoint: []string{
				"sh",
				"-c",
				"mount --make-shared / && " +
					"mount --make-shared /var/lib/docker && " +
					"dockerd-entrypoint.sh --storage-driver=" + storageDriver,
			},
			Hostname: dindAddress,
		}
		hostCfg := &container.HostConfig{
			Privileged: true,
			Binds:      binds,
			// this gets ignored if resolv.conf is bind mounted. So it's ok to have it anyway.
			DNS: []string{dindDNS},
			// Calico needs this
			Sysctls: map[string]string{
				"net.ipv4.conf.all.rp_filter": "1",
			},
		}
		resp, err := cli.ContainerCreate(ctx, imageCfg, hostCfg, nil, containerName)
		if err != nil {
			return "", fmt.Errorf("Failed to create [%s] container on host [%s]: %v", containerName, cli.DaemonHost(), err)
		}

		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			return "", fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, cli.DaemonHost(), err)
		}
		logrus.Infof("[%s] Successfully started [%s] container on host [%s]", DINDPlane, containerName, cli.DaemonHost())
		dindContainer, err := cli.ContainerInspect(ctx, containerName)
		if err != nil {
			return "", fmt.Errorf("Failed to get the address of container [%s] on host [%s]: %v", containerName, cli.DaemonHost(), err)
		}
		dindIPAddress := dindContainer.NetworkSettings.IPAddress

		return dindIPAddress, nil
	}
	dindContainer, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return "", fmt.Errorf("Failed to get the address of container [%s] on host [%s]: %v", containerName, cli.DaemonHost(), err)
	}
	dindIPAddress := dindContainer.NetworkSettings.IPAddress
	logrus.Infof("[%s] container [%s] is already running on host[%s]", DINDPlane, containerName, cli.DaemonHost())
	return dindIPAddress, nil
}

func RmoveDindContainer(ctx context.Context, dindAddress string) error {
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	containerName := fmt.Sprintf("%s-%s", DINDContainerPrefix, dindAddress)
	logrus.Infof("[%s] Removing dind container [%s] on host [%s]", DINDPlane, containerName, cli.DaemonHost())
	_, err = cli.ContainerInspect(ctx, containerName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return nil
		}
	}
	if err := cli.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true}); err != nil {
		return fmt.Errorf("Failed to remove dind container [%s] on host [%s]: %v", containerName, cli.DaemonHost(), err)
	}
	logrus.Infof("[%s] Successfully Removed dind container [%s] on host [%s]", DINDPlane, containerName, cli.DaemonHost())
	return nil
}
