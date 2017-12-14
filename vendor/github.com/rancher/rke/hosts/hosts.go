package hosts

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

type Host struct {
	v3.RKEConfigNode
	DClient   *client.Client
	Dialer    Dialer
	IsControl bool
	IsWorker  bool
}

const (
	ToCleanEtcdDir       = "/var/lib/etcd"
	ToCleanSSLDir        = "/etc/kubernetes/ssl"
	ToCleanCNIConf       = "/etc/cni"
	ToCleanCNIBin        = "/opt/cni"
	ToCleanCalicoRun     = "/var/run/calico"
	CleanerContainerName = "kube-cleaner"
)

func (h *Host) CleanUpAll(cleanerImage string) error {
	logrus.Infof("[hosts] Cleaning up host [%s]", h.Address)
	toCleanPaths := []string{
		ToCleanEtcdDir,
		ToCleanSSLDir,
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
	}
	return h.CleanUp(toCleanPaths, cleanerImage)
}

func (h *Host) CleanUpWorkerHost(controlRole, cleanerImage string) error {
	if h.IsControl {
		logrus.Infof("[hosts] Host [%s] is already a controlplane host, skipping cleanup.", h.Address)
		return nil
	}
	toCleanPaths := []string{
		ToCleanSSLDir,
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
	}
	return h.CleanUp(toCleanPaths, cleanerImage)
}

func (h *Host) CleanUpControlHost(workerRole, cleanerImage string) error {
	if h.IsWorker {
		logrus.Infof("[hosts] Host [%s] is already a worker host, skipping cleanup.", h.Address)
		return nil
	}
	toCleanPaths := []string{
		ToCleanSSLDir,
		ToCleanCNIConf,
		ToCleanCNIBin,
		ToCleanCalicoRun,
	}
	return h.CleanUp(toCleanPaths, cleanerImage)
}

func (h *Host) CleanUp(toCleanPaths []string, cleanerImage string) error {
	logrus.Infof("[hosts] Cleaning up host [%s]", h.Address)
	imageCfg, hostCfg := buildCleanerConfig(h, toCleanPaths, cleanerImage)
	logrus.Infof("[hosts] Running cleaner container on host [%s]", h.Address)
	if err := docker.DoRunContainer(h.DClient, imageCfg, hostCfg, CleanerContainerName, h.Address, CleanerContainerName); err != nil {
		return err
	}

	if err := docker.WaitForContainer(h.DClient, CleanerContainerName); err != nil {
		return err
	}

	logrus.Infof("[hosts] Removing cleaner container on host [%s]", h.Address)
	if err := docker.RemoveContainer(h.DClient, h.Address, CleanerContainerName); err != nil {
		return err
	}
	logrus.Infof("[hosts] Successfully cleaned up host [%s]", h.Address)
	return nil
}

func (h *Host) RegisterDialer(customDialer Dialer) error {
	if customDialer == nil {
		logrus.Infof("[ssh] Setup tunnel for host [%s]", h.Address)
		key, err := checkEncryptedKey(h.SSHKey, h.SSHKeyPath)
		if err != nil {
			return fmt.Errorf("Failed to parse the private key: %v", err)
		}
		dialer := &sshDialer{
			host:   h,
			signer: key,
		}
		h.Dialer = dialer
	} else {
		h.Dialer = customDialer
	}
	return nil
}

func DeleteNode(toDeleteHost *Host, kubeClient *kubernetes.Clientset, hasAnotherRole bool) error {
	if hasAnotherRole {
		logrus.Infof("[hosts] host [%s] has another role, skipping delete from kubernetes cluster", toDeleteHost.Address)
		return nil
	}
	logrus.Infof("[hosts] Cordoning host [%s]", toDeleteHost.Address)
	if _, err := k8s.GetNode(kubeClient, toDeleteHost.HostnameOverride); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Warnf("[hosts] Can't find node by name [%s]", toDeleteHost.Address)
			return nil
		}
		return err

	}
	if err := k8s.CordonUncordon(kubeClient, toDeleteHost.HostnameOverride, true); err != nil {
		return err
	}
	logrus.Infof("[hosts] Deleting host [%s] from the cluster", toDeleteHost.Address)
	if err := k8s.DeleteNode(kubeClient, toDeleteHost.HostnameOverride); err != nil {
		return err
	}
	logrus.Infof("[hosts] Successfully deleted host [%s] from the cluster", toDeleteHost.Address)
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
