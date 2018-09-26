package cluster

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func (c *Cluster) SaveClusterState(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig) error {
	if len(c.ControlPlaneHosts) > 0 {
		// Reinitialize kubernetes Client
		var err error
		c.KubeClient, err = k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
		if err != nil {
			return fmt.Errorf("Failed to re-initialize Kubernetes Client: %v", err)
		}
		err = saveClusterCerts(ctx, c.KubeClient, c.Certificates)
		if err != nil {
			return fmt.Errorf("[certificates] Failed to Save Kubernetes certificates: %v", err)
		}
		err = saveStateToKubernetes(ctx, c.KubeClient, c.LocalKubeConfigPath, rkeConfig)
		if err != nil {
			return fmt.Errorf("[state] Failed to save configuration state to k8s: %v", err)
		}
		// save state to the cluster nodes as a backup
		uniqueHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
		err = saveStateToNodes(ctx, uniqueHosts, rkeConfig, c.SystemImages.Alpine, c.PrivateRegistriesMap)
		if err != nil {
			return fmt.Errorf("[state] Failed to save configuration state to nodes: %v", err)
		}
	}
	return nil
}

func (c *Cluster) GetClusterState(ctx context.Context) (*Cluster, error) {
	var err error
	var currentCluster *Cluster

	// check if local kubeconfig file exists
	if _, err = os.Stat(c.LocalKubeConfigPath); !os.IsNotExist(err) {
		log.Infof(ctx, "[state] Found local kube config file, trying to get state from cluster")

		// to handle if current local admin is down and we need to use new cp from the list
		if !isLocalConfigWorking(ctx, c.LocalKubeConfigPath, c.K8sWrapTransport) {
			if err := rebuildLocalAdminConfig(ctx, c); err != nil {
				return nil, err
			}
		}

		// initiate kubernetes client
		c.KubeClient, err = k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
		if err != nil {
			log.Warnf(ctx, "Failed to initiate new Kubernetes Client: %v", err)
			return nil, nil
		}
		// Get previous kubernetes state
		currentCluster, err = getStateFromKubernetes(ctx, c.KubeClient, c.LocalKubeConfigPath)
		if err != nil {
			// attempting to fetch state from nodes
			uniqueHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)
			currentCluster = getStateFromNodes(ctx, uniqueHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap)
		}
		// Get previous kubernetes certificates
		if currentCluster != nil {
			if err := currentCluster.InvertIndexHosts(); err != nil {
				return nil, fmt.Errorf("Failed to classify hosts from fetched cluster: %v", err)
			}
			activeEtcdHosts := currentCluster.EtcdHosts
			for _, inactiveHost := range c.InactiveHosts {
				activeEtcdHosts = removeFromHosts(inactiveHost, activeEtcdHosts)
			}
			currentCluster.Certificates, err = getClusterCerts(ctx, c.KubeClient, activeEtcdHosts)
			// if getting certificates from k8s failed then we attempt to fetch the backup certs
			if err != nil {
				backupHosts := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, nil)
				currentCluster.Certificates, err = fetchBackupCertificates(ctx, backupHosts, c)
				if err != nil {
					return nil, fmt.Errorf("Failed to Get Kubernetes certificates: %v", err)
				}
				if currentCluster.Certificates != nil {
					log.Infof(ctx, "[certificates] Certificate backup found on backup hosts")
				}
			}
			currentCluster.DockerDialerFactory = c.DockerDialerFactory
			currentCluster.LocalConnDialerFactory = c.LocalConnDialerFactory

			// make sure I have all the etcd certs, We need handle dialer failure for etcd nodes https://github.com/rancher/rancher/issues/12898
			for _, host := range activeEtcdHosts {
				certName := pki.GetEtcdCrtName(host.InternalAddress)
				if (currentCluster.Certificates[certName] == pki.CertificatePKI{}) {
					if currentCluster.Certificates, err = pki.RegenerateEtcdCertificate(ctx,
						currentCluster.Certificates,
						host,
						activeEtcdHosts,
						currentCluster.ClusterDomain,
						currentCluster.KubernetesServiceIP); err != nil {
						return nil, err
					}
				}
			}
			// setting cluster defaults for the fetched cluster as well
			currentCluster.setClusterDefaults(ctx)

			currentCluster.Certificates, err = regenerateAPICertificate(c, currentCluster.Certificates)
			if err != nil {
				return nil, fmt.Errorf("Failed to regenerate KubeAPI certificate %v", err)
			}
		}
	}
	return currentCluster, nil
}

func saveStateToKubernetes(ctx context.Context, kubeClient *kubernetes.Clientset, kubeConfigPath string, rkeConfig *v3.RancherKubernetesEngineConfig) error {
	log.Infof(ctx, "[state] Saving cluster state to Kubernetes")
	clusterFile, err := yaml.Marshal(*rkeConfig)
	if err != nil {
		return err
	}
	timeout := make(chan bool, 1)
	go func() {
		for {
			_, err := k8s.UpdateConfigMap(kubeClient, clusterFile, StateConfigMapName)
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			log.Infof(ctx, "[state] Successfully Saved cluster state to Kubernetes ConfigMap: %s", StateConfigMapName)
			timeout <- true
			break
		}
	}()
	select {
	case <-timeout:
		return nil
	case <-time.After(time.Second * UpdateStateTimeout):
		return fmt.Errorf("[state] Timeout waiting for kubernetes to be ready")
	}
}

func saveStateToNodes(ctx context.Context, uniqueHosts []*hosts.Host, clusterState *v3.RancherKubernetesEngineConfig, alpineImage string, prsMap map[string]v3.PrivateRegistry) error {
	log.Infof(ctx, "[state] Saving cluster state to cluster nodes")
	clusterFile, err := yaml.Marshal(*clusterState)
	if err != nil {
		return err
	}
	for _, host := range uniqueHosts {
		if err := pki.DeployStateOnPlaneHost(ctx, host, alpineImage, prsMap, string(clusterFile)); err != nil {
			return err
		}
	}
	return nil
}

func getStateFromKubernetes(ctx context.Context, kubeClient *kubernetes.Clientset, kubeConfigPath string) (*Cluster, error) {
	log.Infof(ctx, "[state] Fetching cluster state from Kubernetes")
	var cfgMap *v1.ConfigMap
	var currentCluster Cluster
	var err error
	timeout := make(chan bool, 1)
	go func() {
		for {
			cfgMap, err = k8s.GetConfigMap(kubeClient, StateConfigMapName)
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			log.Infof(ctx, "[state] Successfully Fetched cluster state to Kubernetes ConfigMap: %s", StateConfigMapName)
			timeout <- true
			break
		}
	}()
	select {
	case <-timeout:
		clusterData := cfgMap.Data[StateConfigMapName]
		err := yaml.Unmarshal([]byte(clusterData), &currentCluster)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal cluster data")
		}
		return &currentCluster, nil
	case <-time.After(time.Second * GetStateTimeout):
		log.Infof(ctx, "Timed out waiting for kubernetes cluster to get state")
		return nil, fmt.Errorf("Timeout waiting for kubernetes cluster to get state")
	}
}

func getStateFromNodes(ctx context.Context, uniqueHosts []*hosts.Host, alpineImage string, prsMap map[string]v3.PrivateRegistry) *Cluster {
	log.Infof(ctx, "[state] Fetching cluster state from Nodes")
	var currentCluster Cluster
	var clusterFile string
	var err error

	for _, host := range uniqueHosts {
		filePath := path.Join(host.PrefixPath, pki.TempCertPath, pki.ClusterStateFile)
		clusterFile, err = pki.FetchFileFromHost(ctx, filePath, alpineImage, host, prsMap, pki.StateDeployerContainerName, "state")
		if err == nil {
			break
		}
	}
	if len(clusterFile) == 0 {
		return nil
	}
	err = yaml.Unmarshal([]byte(clusterFile), &currentCluster)
	if err != nil {
		logrus.Debugf("[state] Failed to unmarshal the cluster file fetched from nodes: %v", err)
		return nil
	}
	log.Infof(ctx, "[state] Successfully fetched cluster state from Nodes")
	return &currentCluster

}

func GetK8sVersion(localConfigPath string, k8sWrapTransport k8s.WrapTransport) (string, error) {
	logrus.Debugf("[version] Using %s to connect to Kubernetes cluster..", localConfigPath)
	k8sClient, err := k8s.NewClient(localConfigPath, k8sWrapTransport)
	if err != nil {
		return "", fmt.Errorf("Failed to create Kubernetes Client: %v", err)
	}
	discoveryClient := k8sClient.DiscoveryClient
	logrus.Debugf("[version] Getting Kubernetes server version..")
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("Failed to get Kubernetes server version: %v", err)
	}
	return fmt.Sprintf("%#v", *serverVersion), nil
}
