package dind

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/docker"
	"github.com/sirupsen/logrus"
)

const (
	DINDImage           = "docker:17.03-dind"
	DINDContainerPrefix = "rke-dind-"
	DINDPlane           = "dind"
	DINDNetwork         = "dind-network"
	DINDSubnet          = "172.18.0.0/16"
)

func StartUpDindContainer(ctx context.Context, dindAddress, dindNetwork string) error {
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	// its recommended to use host's storage driver
	dockerInfo, err := cli.Info(ctx)
	if err != nil {
		return err
	}
	storageDriver := dockerInfo.Driver
	// Get dind container name
	containerName := DINDContainerPrefix + dindAddress
	_, err = cli.ContainerInspect(ctx, containerName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return err
		}
		if err := docker.UseLocalOrPull(ctx, cli, cli.DaemonHost(), DINDImage, DINDPlane, nil); err != nil {
			return err
		}
		binds := []string{
			fmt.Sprintf("/var/lib/kubelet-%s:/var/lib/kubelet:shared", containerName),
			"/etc/resolv.conf:/etc/resolv.conf",
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
		}
		hostCfg := &container.HostConfig{
			Privileged: true,
			Binds:      binds,
		}
		netCfg := &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				dindNetwork: &network.EndpointSettings{
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: dindAddress,
					},
				},
			},
		}
		resp, err := cli.ContainerCreate(ctx, imageCfg, hostCfg, netCfg, containerName)
		if err != nil {
			return fmt.Errorf("Failed to create [%s] container on host [%s]: %v", containerName, cli.DaemonHost(), err)
		}

		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("Failed to start [%s] container on host [%s]: %v", containerName, cli.DaemonHost(), err)
		}
		logrus.Infof("[%s] Successfully started [%s] container on host [%s]", DINDPlane, containerName, cli.DaemonHost())
		return nil
	}
	logrus.Infof("[%s] container [%s] is already running on host[%s]", DINDPlane, containerName, cli.DaemonHost())
	return nil
}

func RmoveDindContainer(ctx context.Context, dindAddress string) error {
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	containerName := DINDContainerPrefix + dindAddress
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

func CreateDindNetwork(ctx context.Context, dindSubnet string) error {
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	networkList, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	for _, net := range networkList {
		if DINDNetwork == net.Name {
			subnetFound := false
			for _, netConfig := range net.IPAM.Config {
				if netConfig.Subnet == dindSubnet {
					subnetFound = true
					break
				}
			}
			if !subnetFound {
				return fmt.Errorf("dind network [%s] exist but has different subnet than specified", DINDNetwork)
			}
			logrus.Infof("[%s] dind network [%s] with subnet [%s] already created", DINDPlane, DINDNetwork, dindSubnet)
			return nil
		}
	}
	logrus.Infof("[%s] creating dind network [%s] with subnet [%s]", DINDPlane, DINDNetwork, dindSubnet)
	_, err = cli.NetworkCreate(ctx, DINDNetwork, types.NetworkCreate{
		Driver: "bridge",
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				network.IPAMConfig{
					Subnet: dindSubnet,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	logrus.Infof("[%s] Successfully Created dind network [%s] with subnet [%s]", DINDPlane, DINDNetwork, dindSubnet)
	return nil
}
