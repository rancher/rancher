// +build !windows

package clean

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/hosts"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const ContainerName = "cattle-node-cleanup"

func Job() error {
	logrus.Infof("Starting clean container job: %s", ContainerName)

	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	containerList, err := c.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, c := range containerList {
		for _, n := range c.Names {
			if n == "/"+ContainerName {
				logrus.Infof("container named %s already exists, exiting.", ContainerName)
				return nil
			}
		}
	}

	binds := []string{
		"/var/run/docker.sock:/var/run/docker.sock",
		"/:/host:z",
	}

	container, err := c.ContainerCreate(ctx, &container.Config{
		Image: getAgentImage(),
		Env: []string{
			"AGENT_IMAGE=" + getAgentImage(),
			"PREFIX_PATH=" + os.Getenv("PREFIX_PATH"),
			"WINDOWS_PREFIX_PATH=" + os.Getenv("WINDOWS_PREFIX_PATH"),
		},
		Cmd: []string{"--", "agent", "clean"},
	}, &container.HostConfig{
		Binds:       binds,
		Privileged:  true,
		NetworkMode: "host",
	}, &network.NetworkingConfig{}, ContainerName)

	if err != nil {
		return err
	}

	return c.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
}

func Node() error {
	logrus.Info("Cleaning up node...")

	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()

	if err := waitForK8sPods(ctx, c); err != nil {
		return fmt.Errorf("error waiting for k8s pods to be removed: %s", err)
	}

	if err := stopContainers(ctx, c); err != nil {
		return fmt.Errorf("error trying to stop all rancher containers: %s", err)
	}

	if err := cleanDocker(ctx, c); err != nil {
		return fmt.Errorf("error trying to system prune docker: %s", err)
	}

	if err := Links(); err != nil {
		return fmt.Errorf("error trying to clean links from the host: %s", err)
	}

	if err := Firewall(); err != nil {
		return fmt.Errorf("error trying to flush iptables rules: %s", err)
	}

	return Paths()
}

func Links() error {
	for _, l := range []string{"flannel.1", "cni0", "tunl0", "weave", "datapath", "vxlan-6784"} {
		logrus.Infof("checking for link %s", l)
		existing, err := netlink.LinkByName(l)
		if err != nil {
			if err.Error() == "Link not found" {
				logrus.Infof("link %s not found", l)
				continue // not found, nothing to do
			}

			return err
		}

		logrus.Infof("found link and will remove: %s", l)
		if err := netlink.LinkDel(existing); err != nil {
			return fmt.Errorf("failed to delete interface: %v", err)
		}
		logrus.Infof("link deleted: %s", l)
	}

	return nil
}

func Paths() error {
	logrus.Info("Cleaning up paths...")

	if err := umountTmpfs(); err != nil {
		return err
	}

	paths := getPaths()
	for _, p := range paths {
		hostPath := path.Join("/host", p)
		logrus.Infof("trying to delete path: %s", hostPath)
		_, err := os.Stat(hostPath)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Infof("path does not exist: %s", hostPath)
				continue
			}
			return err
		}

		err = os.RemoveAll(hostPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func umountTmpfs() error {
	// umount tmpfs kubelet stuffs
	tmpfs, err := exec.Command(
		"sh", "-c", "mount | grep tmpfs | grep '/var/lib/kubelet' | awk '{ print $3 }'").Output()
	if err != nil {
		return err
	}

	for _, t := range strings.Split(strings.TrimSpace(string(tmpfs)), "\n") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}

		logrus.Infof("trying to umount tmpfs: %s", t)
		_, err := exec.Command("sh", "-c", fmt.Sprintf("umount %s", t)).Output()
		if err != nil {
			return fmt.Errorf("error trying to umount tmpfs %s: %s", t, err)
		}
		logrus.Infof("umount of tmpfs %s successful", t)
	}

	return nil
}

func getAgentImage() string {
	agentImage := os.Getenv("AGENT_IMAGE")
	if agentImage == "" {
		agentImage = "rancher/rancher-agent:master"
	}
	return agentImage
}

func cleanDocker(ctx context.Context, c *client.Client) error {
	blankArgs := filters.NewArgs()
	if _, err := c.ContainersPrune(ctx, blankArgs); err != nil {
		return err
	}
	if _, err := c.VolumesPrune(ctx, blankArgs); err != nil {
		return err
	}
	if _, err := c.ImagesPrune(ctx, blankArgs); err != nil {
		return err
	}
	if _, err := c.NetworksPrune(ctx, blankArgs); err != nil {
		return err
	}

	return nil
}

func getPrefixPath() string {
	return os.Getenv("PREFIX_PATH")
}

func stopContainers(ctx context.Context, c *client.Client) error {
	containers, err := c.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, container := range containers {
		isCleanup := false
		for _, n := range container.Names {
			if strings.HasPrefix(n, "/"+ContainerName) {
				isCleanup = true
				break
			}
		}

		if isCleanup {
			continue // don't stop the cleanup container!
		}

		config, err := c.ContainerInspect(ctx, container.ID)
		if err != nil {
			return err
		}
		if strings.HasPrefix(config.Config.Image, "rancher/") {
			if err := c.ContainerKill(ctx, config.ID, "SIGKILL"); err != nil {
				return err
			}
		}
	}

	return nil
}

func waitForK8sPods(ctx context.Context, c *client.Client) error {
	// wait for up to 5min for k8s pods to be dropped
	for i := 0; i < 30; i++ {
		logrus.Infof("checking for pods %d out of 30 times", i)
		containerList, err := c.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			return err
		}

		hasPods := false
		for _, c := range containerList {
			for _, n := range c.Names {
				if strings.HasPrefix(n, "/k8s_") {
					hasPods = true
					continue
				}
			}
			if hasPods {
				continue //break out if you already found one
			}
		}

		if hasPods {
			logrus.Info("pods found, waiting 10s and trying again")
			time.Sleep(10 * time.Second)
			continue
		}

		logrus.Info("all pods cleaned, continuing on to more rke cleanup")
		return nil
	}

	return nil
}

func Firewall() error {
	// save docker rules
	if _, err := exec.Command("sh", "-c",
		"iptables-save -t filter | egrep -w \"COMMIT|DOCKER|docker0|\\*filter\" > filter.txt").Output(); err != nil {
		return err
	}

	if _, err := exec.Command("sh", "-c",
		"iptables-save -t nat | egrep -w \"COMMIT|DOCKER|docker0|\\*nat\" > nat.txt").Output(); err != nil {
		return err
	}

	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	for _, table := range []string{"nat", "mangle"} {
		logrus.Infof("clearing and deleting iptables table: %s", table)
		if err := ipt.ClearAndDeleteChain(table, ""); err != nil {
			return err
		}
	}

	if err := ipt.ClearAll(); err != nil {
		return err
	}
	if err := ipt.DeleteAll(); err != nil {
		return err
	}

	if _, err := exec.Command("sh", "-c",
		"iptables-restore filter.txt").Output(); err != nil {
		return fmt.Errorf("error putting back filter table: %s", err)
	}

	if _, err := exec.Command("sh", "-c",
		"iptables-restore nat.txt").Output(); err != nil {
		return fmt.Errorf("error putting back nat table: %s", err)
	}

	return nil
}

func getPaths() []string {
	pathPrefix := getPrefixPath()

	// all paths on the hosts to rm -rf
	return []string{
		hosts.ToCleanCNIConf, hosts.ToCleanCNIBin,
		"/opt/rke", "/run/secrets/kubernetes.io",
		"/run/calico", "/run/flannel", "/var/log/containers", "/var/log/pods",
		hosts.ToCleanCalicoRun,
		path.Join(pathPrefix, hosts.ToCleanSSLDir),
		path.Join(pathPrefix, hosts.ToCleanTempCertPath),
		path.Join(pathPrefix, hosts.ToCleanEtcdDir),
		path.Join(pathPrefix, hosts.ToCleanCNILib),
		path.Join(pathPrefix, "/var/lib/kubelet"),
		path.Join(pathPrefix, "/var/lib/calico"),
		path.Join(pathPrefix, "/var/lib/weave"),
		path.Join(pathPrefix, "/etc/ceph"),
	}
}
