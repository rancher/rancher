package hosts

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
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
}

const (
	ToCleanEtcdDir       = "/var/lib/etcd"
	ToCleanSSLDir        = "/etc/kubernetes/ssl"
	ToCleanCNIConf       = "/etc/cni"
	ToCleanCNIBin        = "/opt/cni"
	ToCleanCalicoRun     = "/var/run/calico"
	ToCleanTempCertPath  = "/etc/kubernetes/.tmp/"
	CleanerContainerName = "kube-cleaner"
)

func (h *Host) CleanUpAll(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	log.Infof(ctx, "[hosts] Cleaning up host [%s]", h.Address)
	toCleanPaths := []string{
		ToCleanEtcdDir,
		ToCleanSSLDir,
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
		ToCleanTempCertPath,
	}
	return h.CleanUp(ctx, toCleanPaths, cleanerImage, prsMap)
}

func (h *Host) CleanUpWorkerHost(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	if h.IsControl {
		log.Infof(ctx, "[hosts] Host [%s] is already a controlplane host, skipping cleanup.", h.Address)
		return nil
	}
	toCleanPaths := []string{
		ToCleanSSLDir,
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
	}
	return h.CleanUp(ctx, toCleanPaths, cleanerImage, prsMap)
}

func (h *Host) CleanUpControlHost(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	if h.IsWorker {
		log.Infof(ctx, "[hosts] Host [%s] is already a worker host, skipping cleanup.", h.Address)
		return nil
	}
	toCleanPaths := []string{
		ToCleanSSLDir,
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
	}
	return h.CleanUp(ctx, toCleanPaths, cleanerImage, prsMap)
}

func (h *Host) CleanUpEtcdHost(ctx context.Context, cleanerImage string, prsMap map[string]v3.PrivateRegistry) error {
	toCleanPaths := []string{
		ToCleanEtcdDir,
		ToCleanSSLDir,
	}
	if h.IsWorker || h.IsControl {
		log.Infof(ctx, "[hosts] Host [%s] is already a worker or control host, skipping cleanup certs.", h.Address)
		toCleanPaths = []string{
			ToCleanEtcdDir,
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

	if err := docker.WaitForContainer(ctx, h.DClient, CleanerContainerName); err != nil {
		return err
	}

	log.Infof(ctx, "[hosts] Removing cleaner container on host [%s]", h.Address)
	if err := docker.RemoveContainer(ctx, h.DClient, h.Address, CleanerContainerName); err != nil {
		return err
	}
	log.Infof(ctx, "[hosts] Successfully cleaned up host [%s]", h.Address)
	return nil
}

func DeleteNode(ctx context.Context, toDeleteHost *Host, kubeClient *kubernetes.Clientset, hasAnotherRole bool) error {
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
	if err := k8s.DeleteNode(kubeClient, toDeleteHost.HostnameOverride); err != nil {
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

func GetToDeleteHosts(currentHosts, configHosts []*Host) []*Host {
	toDeleteHosts := []*Host{}
	for _, currentHost := range currentHosts {
		found := false
		for _, newHost := range configHosts {
			if currentHost.Address == newHost.Address {
				found = true
			}
		}
		if !found {
			toDeleteHosts = append(toDeleteHosts, currentHost)
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
	cmd := append([]string{"rm", "-rf"}, toCleanDirs...)
	imageCfg := &container.Config{
		Image: cleanerImage,
		Cmd:   cmd,
	}
	bindMounts := []string{}
	for _, vol := range toCleanDirs {
		bindMounts = append(bindMounts, fmt.Sprintf("%s:%s", vol, vol))
	}
	hostCfg := &container.HostConfig{
		Binds: bindMounts,
	}
	return imageCfg, hostCfg
}
