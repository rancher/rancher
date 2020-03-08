package hosts

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"

	"github.com/docker/docker/client"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

type Host struct {
	v3.RKEConfigNode
	DClient             *client.Client
	LocalConnPort       int
	IsControl           bool
	IsWorker            bool
	IsEtcd              bool
	IgnoreDockerVersion bool
	ToAddEtcdMember     bool
	ExistingEtcdCluster bool
	SavedKeyPhrase      string
	ToAddLabels         map[string]string
	ToDelLabels         map[string]string
	ToAddTaints         []string
	ToDelTaints         []string
	DockerInfo          types.Info
	UpdateWorker        bool
	PrefixPath          string
	BastionHost         v3.BastionHost
}

const (
	ToCleanEtcdDir          = "/var/lib/etcd/"
	ToCleanSSLDir           = "/etc/kubernetes/"
	ToCleanCNIConf          = "/etc/cni/"
	ToCleanCNIBin           = "/opt/cni/"
	ToCleanCNILib           = "/var/lib/cni/"
	ToCleanCalicoRun        = "/var/run/calico/"
	ToCleanTempCertPath     = "/etc/kubernetes/.tmp/"
	CleanerContainerName    = "kube-cleaner"
	LogCleanerContainerName = "rke-log-cleaner"
	RKELogsPath             = "/var/lib/rancher/rke/log"

	B2DOS             = "Boot2Docker"
	B2DPrefixPath     = "/mnt/sda1/rke"
	ROS               = "RancherOS"
	ROSPrefixPath     = "/opt/rke"
	CoreOS            = "CoreOS"
	CoreOSPrefixPath  = "/opt/rke"
	WindowsOS         = "Windows"
	WindowsPrefixPath = "c:/"
)

func (h *Host) CleanUpAll(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry, externalEtcd bool) error {
	log.Infof(ctx, "[hosts] Cleaning up host [%s]", h.Address)
	toCleanPaths := []string{
		path.Join(h.PrefixPath, ToCleanSSLDir),
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
		path.Join(h.PrefixPath, ToCleanTempCertPath),
		path.Join(h.PrefixPath, ToCleanCNILib),
	}

	if !externalEtcd {
		toCleanPaths = append(toCleanPaths, path.Join(h.PrefixPath, ToCleanEtcdDir))
	}
	return h.CleanUp(ctx, toCleanPaths, cleanerImage, prsMap)
}

func (h *Host) CleanUpWorkerHost(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	if h.IsControl || h.IsEtcd {
		log.Infof(ctx, "[hosts] Host [%s] is already a controlplane or etcd host, skipping cleanup.", h.Address)
		return nil
	}
	toCleanPaths := []string{
		path.Join(h.PrefixPath, ToCleanSSLDir),
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
		path.Join(h.PrefixPath, ToCleanCNILib),
	}
	return h.CleanUp(ctx, toCleanPaths, cleanerImage, prsMap)
}

func (h *Host) CleanUpControlHost(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	if h.IsWorker || h.IsEtcd {
		log.Infof(ctx, "[hosts] Host [%s] is already a worker or etcd host, skipping cleanup.", h.Address)
		return nil
	}
	toCleanPaths := []string{
		path.Join(h.PrefixPath, ToCleanSSLDir),
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
		path.Join(h.PrefixPath, ToCleanCNILib),
	}
	return h.CleanUp(ctx, toCleanPaths, cleanerImage, prsMap)
}

func (h *Host) CleanUpEtcdHost(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	toCleanPaths := []string{
		path.Join(h.PrefixPath, ToCleanEtcdDir),
		path.Join(h.PrefixPath, ToCleanSSLDir),
	}
	if h.IsWorker || h.IsControl {
		log.Infof(ctx, "[hosts] Host [%s] is already a worker or control host, skipping cleanup certs.", h.Address)
		toCleanPaths = []string{
			path.Join(h.PrefixPath, ToCleanEtcdDir),
		}
	}
	return h.CleanUp(ctx, toCleanPaths, cleanerImage, prsMap)
}

func (h *Host) CleanUp(ctx context.Context, toCleanPaths []string, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	log.Infof(ctx, "[hosts] Cleaning up host [%s]", h.Address)
	imageCfg, hostCfg := buildCleanerConfig(h, toCleanPaths, cleanerImage)
	log.Infof(ctx, "[hosts] Running cleaner container on host [%s]", h.Address)
	if err := docker.DoRunContainer(ctx, h.DClient, imageCfg, hostCfg, CleanerContainerName, h.Address, CleanerContainerName, prsMap); err != nil {
		return err
	}

	if _, err := docker.WaitForContainer(ctx, h.DClient, h.Address, CleanerContainerName); err != nil {
		return err
	}

	log.Infof(ctx, "[hosts] Removing cleaner container on host [%s]", h.Address)
	if err := docker.RemoveContainer(ctx, h.DClient, h.Address, CleanerContainerName); err != nil {
		return err
	}
	log.Infof(ctx, "[hosts] Removing dead container logs on host [%s]", h.Address)
	if err := DoRunLogCleaner(ctx, h, cleanerImage, prsMap); err != nil {
		return err
	}
	log.Infof(ctx, "[hosts] Successfully cleaned up host [%s]", h.Address)
	return nil
}

func DeleteNode(ctx context.Context, toDeleteHost *Host, kubeClient *kubernetes.Clientset, hasAnotherRole bool, cloudProvider string) error {
	if hasAnotherRole {
		log.Infof(ctx, "[hosts] host [%s] has another role, skipping delete from kubernetes cluster", toDeleteHost.Address)
		return nil
	}
	log.Infof(ctx, "[hosts] Cordoning host [%s]", toDeleteHost.Address)
	if _, err := k8s.GetNode(kubeClient, toDeleteHost.HostnameOverride); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warnf(ctx, "[hosts] Can't find node by name [%s]", toDeleteHost.Address)
			return nil
		}
		return err

	}
	if err := k8s.CordonUncordon(kubeClient, toDeleteHost.HostnameOverride, true); err != nil {
		return err
	}
	log.Infof(ctx, "[hosts] Deleting host [%s] from the cluster", toDeleteHost.Address)
	if err := k8s.DeleteNode(kubeClient, toDeleteHost.HostnameOverride, cloudProvider); err != nil {
		return err
	}
	log.Infof(ctx, "[hosts] Successfully deleted host [%s] from the cluster", toDeleteHost.Address)
	return nil
}

func RemoveTaintFromHost(ctx context.Context, host *Host, taintKey string, kubeClient *kubernetes.Clientset) error {
	log.Infof(ctx, "[hosts] removing taint [%s] from host [%s]", taintKey, host.Address)
	if err := k8s.RemoveTaintFromNodeByKey(kubeClient, host.HostnameOverride, taintKey); err != nil {
		return err
	}
	log.Infof(ctx, "[hosts] Successfully deleted taint [%s] from host [%s]", taintKey, host.Address)
	return nil
}

func GetToDeleteHosts(currentHosts, configHosts, inactiveHosts []*Host, includeInactive bool) []*Host {
	toDeleteHosts := []*Host{}
	for _, currentHost := range currentHosts {
		found := false
		for _, newHost := range configHosts {
			if currentHost.Address == newHost.Address {
				found = true
			}
		}
		if !found {
			inactive := false
			for _, inactiveHost := range inactiveHosts {
				if inactiveHost.Address == currentHost.Address {
					inactive = true
					break
				}
			}
			if (inactive && includeInactive) || !inactive {
				toDeleteHosts = append(toDeleteHosts, currentHost)
			}
		}
	}
	return toDeleteHosts
}

func GetToAddHosts(currentHosts, configHosts []*Host) []*Host {
	toAddHosts := []*Host{}
	for _, configHost := range configHosts {
		found := false
		for _, currentHost := range currentHosts {
			if currentHost.Address == configHost.Address {
				found = true
				break
			}
		}
		if !found {
			toAddHosts = append(toAddHosts, configHost)
		}
	}
	return toAddHosts
}

func IsHostListChanged(currentHosts, configHosts []*Host) bool {
	changed := false
	for _, host := range currentHosts {
		found := false
		for _, configHost := range configHosts {
			if host.Address == configHost.Address {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	for _, host := range configHosts {
		found := false
		for _, currentHost := range currentHosts {
			if host.Address == currentHost.Address {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return changed
}

func buildCleanerConfig(host *Host, toCleanDirs []string, cleanerImage string) (*container.Config, *container.HostConfig) {
	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf("find %s -mindepth 1 -delete", strings.Join(toCleanDirs, " ")),
	}
	imageCfg := &container.Config{
		Image: cleanerImage,
		Cmd:   cmd,
	}
	bindMounts := []string{}
	for _, vol := range toCleanDirs {
		bindMounts = append(bindMounts, fmt.Sprintf("%s:%s:z", vol, vol))
	}
	hostCfg := &container.HostConfig{
		Binds: bindMounts,
	}
	return imageCfg, hostCfg
}

func NodesToHosts(rkeNodes []v3.RKEConfigNode, nodeRole string) []*Host {
	hostList := make([]*Host, 0)
	// Return all nodes if there is no noderole passed to the function
	if nodeRole == "" {
		for _, node := range rkeNodes {
			newHost := Host{
				RKEConfigNode: node,
			}
			hostList = append(hostList, &newHost)
		}
		return hostList
	}
	for _, node := range rkeNodes {
		for _, role := range node.Role {
			if role == nodeRole {
				newHost := Host{
					RKEConfigNode: node,
				}
				hostList = append(hostList, &newHost)
				break
			}
		}
	}
	return hostList
}

func GetUniqueHostList(etcdHosts, cpHosts, workerHosts []*Host) []*Host {
	hostList := []*Host{}
	hostList = append(hostList, etcdHosts...)
	hostList = append(hostList, cpHosts...)
	hostList = append(hostList, workerHosts...)
	// little trick to get a unique host list
	uniqHostMap := make(map[*Host]bool)
	for _, host := range hostList {
		uniqHostMap[host] = true
	}
	uniqHostList := []*Host{}
	for host := range uniqHostMap {
		uniqHostList = append(uniqHostList, host)
	}
	return uniqHostList
}

func GetPrefixPath(osType, ClusterPrefixPath string) string {
	var prefixPath string
	switch {
	case ClusterPrefixPath != "/":
		prefixPath = ClusterPrefixPath
	case strings.Contains(osType, B2DOS):
		prefixPath = B2DPrefixPath
	case strings.Contains(osType, ROS):
		prefixPath = ROSPrefixPath
	case strings.Contains(osType, CoreOS):
		prefixPath = CoreOSPrefixPath
	case strings.Contains(osType, WindowsOS):
		prefixPath = WindowsPrefixPath
	default:
		prefixPath = ClusterPrefixPath
	}
	return prefixPath
}

func DoRunLogCleaner(ctx context.Context, host *Host, alpineImage string, prsMap map[string]v3.PrivateRegistry) error {
	logrus.Debugf("[cleanup] Starting log link cleanup on host [%s]", host.Address)
	imageCfg := &container.Config{
		Image: alpineImage,
		Tty:   true,
		Cmd: []string{
			"sh",
			"-c",
			fmt.Sprintf("find %s -type l ! -exec test -e {} \\; -print -delete", RKELogsPath),
		},
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			host.DockerInfo.DockerRootDir + ":" + host.DockerInfo.DockerRootDir,
			"/var/lib:/var/lib",
		},
		Privileged: true,
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, LogCleanerContainerName, host.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, LogCleanerContainerName, host.Address, "cleanup", prsMap); err != nil {
		return err
	}
	if err := docker.DoRemoveContainer(ctx, host.DClient, LogCleanerContainerName, host.Address); err != nil {
		return err
	}
	logrus.Debugf("[cleanup] Successfully cleaned up log links on host [%s]", host.Address)
	return nil
}

func IsNodeInList(host *Host, hostList []*Host) bool {
	for _, h := range hostList {
		if h.HostnameOverride == host.HostnameOverride {
			return true
		}
	}
	return false
}

func GetHostListIntersect(a []*Host, b []*Host) []*Host {
	s := []*Host{}
	hash := map[string]*Host{}
	for _, h := range a {
		hash[h.Address] = h
	}
	for _, h := range b {
		if _, ok := hash[h.Address]; ok {
			s = append(s, h)
		}
	}
	return s
}

func GetInternalAddressForHosts(hostList []*Host) []string {
	hostAddresses := []string{}
	for _, host := range hostList {
		hostAddresses = append(hostAddresses, host.InternalAddress)
	}
	return hostAddresses
}
