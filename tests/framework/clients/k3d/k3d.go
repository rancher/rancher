package k3d

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/wrangler/pkg/randomtoken"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateK3DCluster creates a minimal k3d cluster and returns a rest config for connecting to the newly created cluster.
//  If a name is given a random one will be generated.
func CreateK3DCluster(ts *session.Session, name string) (*rest.Config, error) {
	k3dConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, k3dConfig)

	name = defaultName(name)

	ts.RegisterCleanupFunc(func() error {
		return DeleteK3DCluster(name)
	})

	msg, err := exec.Command("k3d", "cluster", "create", name,
		"--no-lb",
		"--no-hostip",
		"--no-image-volume",
		"--kubeconfig-update-default=false",
		"--kubeconfig-switch-context=false",
		fmt.Sprintf("--timeout=%d", k3dConfig.createTimeout),
		`--k3s-server-arg=--no-deploy=traefik`,
		`--k3s-server-arg=--no-deploy=servicelb`,
		`--k3s-server-arg=--no-deploy=metrics-serve`,
		`--k3s-server-arg=--no-deploy=local-storage`).CombinedOutput()
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
func CreateAndImportK3DCluster(client *rancher.Client, name string) (*management.Cluster, error) {
	var err error

	name = defaultName(name)

	// create the provisioning cluster
	cluster := &apisV1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "fleet-default",
		},
	}
	_, err = client.Provisioning.Clusters("fleet-default").Create(context.TODO(), cluster, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to create provisioning cluster")
	}

	// create the k3s cluster
	downRest, err := CreateK3DCluster(client.Session, name)
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to create k3d cluster")
	}

	// wait for the management cluster
	mClusterWatch, err := client.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to watch for management cluster")
	}

	var mgmtCluster *management.Cluster
	err = wait.WatchWait(mClusterWatch, func(event watch.Event) (bool, error) {
		var mc v3.Cluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(event.Object.(*unstructured.Unstructured).Object, &mc)
		if mc.Spec.DisplayName == name {
			mgmtCluster, err = client.Management.Cluster.ByID(mc.Name)
			return true, err
		}

		return false, nil

	})
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to watch for management cluster")
	}

	// import the k3d cluster
	err = clusters.ImportCluster(client, mgmtCluster, downRest)
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to import cluster")
	}

	// wait for cluster to be ready
	mClusterWatch, err = client.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to wait for management cluster ready")
	}

	err = wait.WatchWait(mClusterWatch, func(event watch.Event) (bool, error) {
		var mc v3.Cluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(event.Object.(*unstructured.Unstructured).Object, &mc)

		_ = scheme.Scheme.Convert(event.Object, mgmtCluster, nil)

		for _, cond := range mc.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				return true, nil
			}

			break
		}

		return false, nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "CreateAndImportK3DCluster: failed to wait for management cluster ready")
	}

	return mgmtCluster, nil
}

// defaultName returns a random string if name is empty, otherwise name is returned unmodified.
func defaultName(name string) string {
	if name == "" {
		name, _ = randomtoken.Generate()
	}

	return name[:8]
}
