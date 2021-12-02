//go:build !windows
// +build !windows

package clean

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/hosts"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const (
	DockerPipe = "/var/run/docker.sock"
	HostMount  = "/host"
)

func Run(ctx context.Context, args []string) error {
	if len(args) > 3 {
		fmt.Println(usage())
		return nil
	}

	if len(args) == 3 {
		switch args[2] {
		case "job":
			return job(ctx)
		case "node":
			return Node(ctx)
		case "link", "links":
			return links()
		case "cluster":
			return Cluster()
		case "path", "paths":
			return paths()
		case "firewall", "firewalls":
			return firewall()
		case "script", "scripts":
			fmt.Print(script())
			return nil
		case "help":
			fmt.Println(usage())
			return nil
		default:
			fmt.Println(usage())
			return nil
		}
	}

	fmt.Println(usage())
	return nil
}

func job(ctx context.Context) error {
	logrus.Infof("Starting clean container job: %s", NodeCleanupContainerName)

	c, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	containerList, err := c.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, c := range containerList {
		for _, n := range c.Names {
			if n == "/"+NodeCleanupContainerName {
				logrus.Infof("container named %s already exists, exiting.", NodeCleanupContainerName)
				return nil
			}
		}
	}

	binds := []string{
		fmt.Sprintf("%s:%s", DockerPipe, DockerPipe),
		fmt.Sprintf("%s:%s:z", "/", HostMount),
	}

	container, err := c.ContainerCreate(ctx, &container.Config{
		Image: getAgentImage(),
		Env: []string{
			fmt.Sprintf("%s=%s", AgentImage, getAgentImage()),
			fmt.Sprintf("%s=%s", PrefixPath, os.Getenv(PrefixPath)),
			fmt.Sprintf("%s=%s", WindowsPrefixPath, os.Getenv(WindowsPrefixPath)),
		},
		Cmd: []string{"--", "agent", "clean", "node"},
	}, &container.HostConfig{
		AutoRemove:  true,
		Binds:       binds,
		Privileged:  true,
		NetworkMode: "host",
	}, nil, nil, NodeCleanupContainerName)

	if err != nil {
		return err
	}

	return c.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
}

func Node(ctx context.Context) error {
	logrus.Info("Cleaning up node...")

	c, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	p, err := c.Ping(ctx)
	if err != nil {
		return fmt.Errorf("error pinging docker api: %s", err)
	}
	c.NegotiateAPIVersionPing(p)

	if err := waitForK8sPods(ctx, c); err != nil {
		return fmt.Errorf("error waiting for k8s pods to be removed: %s", err)
	}

	if err := containers(ctx, c); err != nil {
		return fmt.Errorf("error trying to stop all rancher containers: %s", err)
	}

	if err := docker(ctx, c); err != nil {
		return fmt.Errorf("error trying to system prune docker: %s", err)
	}

	if err := links(); err != nil {
		return fmt.Errorf("error trying to clean links from the host: %s", err)
	}

	if err := firewall(); err != nil {
		return fmt.Errorf("error trying to flush iptables rules: %s", err)
	}

	return paths()
}

func links() error {
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

func paths() error {
	logrus.Info("Cleaning up paths...")

	if err := umountTmpfs(); err != nil {
		return err
	}

	paths := getPaths()
	for _, p := range paths {
		hostPath := filepath.Join("/host", p)
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

func docker(ctx context.Context, c *client.Client) error {
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

func containers(ctx context.Context, c *client.Client) error {
	containers, err := c.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, container := range containers {
		isCleanup := false
		for _, n := range container.Names {
			if strings.HasPrefix(n, "/"+NodeCleanupContainerName) {
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

func firewall() error {
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

func usage() string {
	return `agent clean usage for linux:
Using the clean commands can be run directly from the agent with proper flags, to clean everything you can run the following:

docker run --privileged --network host -v /:/host -v /var/run/docker.sock:/var/run/docker.sock rancher/rancher-agent:master -- agent clean node

The above mounts and grants access to everything the node cleaner will need to complete
Note: If your cluster was created with a prefixPath use the env param -e PREFIX_PATH=/my/prefix

Running individual cleanup sub commands
docker run --privileged -v /:/host rancher/rancher-agent:master -- agent clean paths

commands:
	node - cleans the entire node and performs all actions below and is the default command, requires docker socket to clean up dockerd
	links - clean the interface links created on the host, requires --privileged and --network host
	paths - clean the paths added by the host like /etc/rancher, /var/lib/rancher, etc. requires -v /:/host 
	firewall - clean the iptables firewall rules, needs --privileged and --network host to work 
	script - prints a bash script you can run to clean up the node from the cli
	help - print this help message

other commands for automation:
	job - used by the k8s batch job to start the ` + NodeCleanupContainerName + ` container to watch kubelet cleanup and wait to clean the node
`
}

func script() string {
	return `#!/bin/bash

# Directories to cleanup
CLEANUP_DIRS=(/etc/ceph /etc/cni /etc/kubernetes /opt/cni /opt/rke /run/secrets/kubernetes.io /run/calico /run/flannel /var/lib/calico /var/lib/weave /var/lib/etcd /var/lib/cni /var/lib/kubelet/* /var/lib/rancher/rke/log /var/log/containers /var/log/pods /var/run/calico)

# Interfaces to cleanup
CLEANUP_INTERFACES=(flannel.1 cni0 tunl0 weave datapath vxlan-6784)

run() {

  CONTAINERS=$(docker ps -qa)
  if [[ -n ${CONTAINERS} ]]
    then
      cleanup-containers
    else
      techo "No containers exist, skipping container cleanup..."
  fi
  cleanup-dirs
  cleanup-interfaces
  VOLUMES=$(docker volume ls -q)
  if [[ -n ${VOLUMES} ]]
    then
      cleanup-volumes
    else
      techo "No volumes exist, skipping container volume cleanup..."
  fi
  if [[ ${CLEANUP_IMAGES} -eq 1 ]]
    then
      IMAGES=$(docker images -q)
      if [[ -n ${IMAGES} ]]
        then
          cleanup-images
        else
          techo "No images exist, skipping container image cleanup..."
      fi
  fi
  if [[ ${FLUSH_IPTABLES} -eq 1 ]]
    then
      flush-iptables
  fi
  techo "Done!"

}

cleanup-containers() {

  techo "Removing containers..."
  docker rm -f $(docker ps -qa)

}

cleanup-dirs() {

  techo "Unmounting filesystems..."
  for mount in $(mount | grep tmpfs | grep '/var/lib/kubelet' | awk '{ print $3 }')
    do
      umount $mount
  done

  techo "Removing directories..."
  for DIR in "${CLEANUP_DIRS[@]}"
    do
      techo "Removing $DIR"
      rm -rf $DIR
  done

}

cleanup-images() {

  techo "Removing images..."
  docker rmi -f $(docker images -q)

}

cleanup-interfaces() {

  techo "Removing interfaces..."
  for INTERFACE in "${CLEANUP_INTERFACES[@]}"
    do
      if $(ip link show ${INTERFACE} > /dev/null 2>&1)
        then
          techo "Removing $INTERFACE"
          ip link delete $INTERFACE
      fi
  done

}

cleanup-volumes() {

  techo "Removing volumes..."
  docker volume rm $(docker volume ls -q)

}

flush-iptables() {

  techo "Flushing iptables..."
  iptables -F -t nat
  iptables -X -t nat
  iptables -F -t mangle
  iptables -X -t mangle
  iptables -F
  iptables -X
  techo "Restarting Docker..."
  if systemctl list-units --full -all | grep -q docker.service
    then
      systemctl restart docker
    else
      /etc/init.d/docker restart
  fi

}

help() {

  echo "Rancher 2.x extended cleanup
  Usage: bash extended-cleanup-rancher2.sh [ -f -i ]

  All flags are optional

  -f | --flush-iptables     Flush all iptables rules (includes a Docker restart)
  -i | --flush-images       Cleanup all container images
  -h                        This help menu

  !! Warning, this script removes containers and all data specific to Kubernetes and Rancher
  !! Backup data as needed before running this script, and use at your own risk."

}

timestamp() {

  date "+%Y-%m-%d %H:%M:%S"

}

techo() {

  echo "$(timestamp): $*"

}

# Check if we're running as root.
if [[ $EUID -ne 0 ]]
  then
    techo "This script must be run as root"
    exit 1
fi

while test $# -gt 0
  do
    case ${1} in
      -f|--flush-iptables)
        shift
        FLUSH_IPTABLES=1
        ;;
      -i|--flush-images)
        shift
        CLEANUP_IMAGES=1
        ;;
      h)
        help && exit 0
        ;;
      *)
        help && exit 0
    esac
done

# Run the cleanup
run
`
}
