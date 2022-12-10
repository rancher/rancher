package k3d

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/wrangler/pkg/randomtoken"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateK3DCluster creates a minimal k3d cluster and returns a rest config for connecting to the newly created cluster.
// If a name is not given a random one will be generated.
func CreateK3DCluster(ts *session.Session, name string) (*rest.Config, error) {
	k3dConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, k3dConfig)

	name = defaultName(name)

	ts.RegisterCleanupFunc(func() error {
		return DeleteK3DCluster(name)
	})

	msg, err := exec.Command("k3d", "cluster", "create", name,
		"--no-lb",
		"--kubeconfig-update-default=false",
		"--kubeconfig-switch-context=false",
		fmt.Sprintf("--timeout=%d", k3dConfig.createTimeout),
		`--k3s-arg=--kubelet-arg=eviction-hard=imagefs.available<1%,nodefs.available<1%`,
		`--k3s-arg=--kubelet-arg=eviction-minimum-reclaim=imagefs.available=1%,nodefs.available=1%`,
		`--k3s-arg=--no-deploy=traefik`,
		`--k3s-arg=--no-deploy=servicelb`,
		`--k3s-arg=--no-deploy=metrics-serve`,
		`--k3s-arg=--no-deploy=local-storage`).CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "CreateK3DCluster: "+string(msg))
	}

	configBytes, err := exec.Command("k3d", "kubeconfig", "get", name).Output()
	if err != nil {
		return nil, errors.Wrap(err, "CreateK3DCluster: failed to get kubeconfig for k3d cluster")
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(configBytes)
	if err != nil {
		return nil, errors.Wrap(err, "CreateK3DCluster: failed to parse kubeconfig for k3d cluster")
	}

	return restConfig, nil
}

// DeleteK3DCluster deletes the k3d cluster with the given name. An error is returned if the cluster does not exist.
func DeleteK3DCluster(name string) error {
	return exec.Command("k3d", "cluster", "delete", name).Run()
}

// CreateAndImportK3DCluster creates a new k3d cluster and imports it into rancher.
func CreateAndImportK3DCluster(client *rancher.Client, name string) (*apisV1.Cluster, error) {
	var err error

	name = defaultName(name)

	// create the provisioning cluster
	cluster := &apisV1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "fleet-default",
		},
	}
	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResouceType).Create(cluster)
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to create provisioning cluster")
	}

	// create the k3s cluster
	downRest, err := CreateK3DCluster(client.Session, name)
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to create k3d cluster")
	}

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to instantiate kube api provisioning client")
	}
	// wait for the imported cluster
	clusterWatch, err := kubeProvisioningClient.Clusters("fleet-default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to watch for the imported cluster")
	}

	var impCluster *apisV1.Cluster
	err = wait.WatchWait(clusterWatch, func(event watch.Event) (bool, error) {
		cluster := event.Object.(*apisV1.Cluster)
		if cluster.Name == name {
			impCluster, err = kubeProvisioningClient.Clusters("fleet-default").Get(context.TODO(), name, metav1.GetOptions{})
			return true, err
		}

		return false, nil

	})
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to watch for management cluster")
	}

	// import the k3d cluster
	err = clusters.ImportCluster(client, impCluster, downRest)
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to import cluster")
	}

	// wait for the imported cluster to be ready
	clusterWatch, err = kubeProvisioningClient.Clusters("fleet-default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})

	checkFunc := clusters.IsProvisioningClusterReady
	err = wait.WatchWait(clusterWatch, checkFunc)

	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to wait for imported cluster ready status")
	}

	return impCluster, nil
}

// defaultName returns a random string if name is empty, otherwise name is returned unmodified.
func defaultName(name string) string {
	if name == "" {
		name, _ = randomtoken.Generate()
	}

	return name[:8]
}
