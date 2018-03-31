package clusterdeploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"

	"io/ioutil"
	"os"
	"path"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/clusteryaml"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	clusterOwnerRole = "cluster-owner"
	tempDir          = "./management-state/tmp"
)

func Register(management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	c := &clusterDeploy{
		systemAccountManager: systemaccount.NewManager(management),
		userManager:          management.UserManager,
		clusters:             management.Management.Clusters(""),
		clusterManager:       clusterManager,
		builder: clusteryaml.NewBuilder(management.Dialer,
			management.Management.Nodes("").Controller().Lister(),
			management.K8sClient.CoreV1()),
	}

	management.Management.Clusters("").AddHandler("cluster-deploy", c.sync)
}

type clusterDeploy struct {
	systemAccountManager *systemaccount.Manager
	userManager          user.Manager
	clusters             v3.ClusterInterface
	clusterManager       *clustermanager.Manager
	builder              *clusteryaml.Builder
}

func (cd *clusterDeploy) sync(key string, cluster *v3.Cluster) error {
	var (
		err, updateErr error
	)

	if key == "" || cluster == nil {
		return nil
	}

	original := cluster
	cluster = original.DeepCopy()

	err = cd.doSync(cluster)
	if cluster != nil && !reflect.DeepEqual(cluster, original) {
		_, updateErr = cd.clusters.Update(cluster)
	}

	if err != nil {
		return err
	}
	return updateErr
}

func (cd *clusterDeploy) doSync(cluster *v3.Cluster) error {
	if !v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil
	}

	if cluster.Spec.Internal && cluster.Status.Driver == v3.ClusterDriverImported {
		return nil
	}

	_, err := v3.ClusterConditionSystemAccountCreated.DoUntilTrue(cluster, func() (runtime.Object, error) {
		return cluster, cd.systemAccountManager.CreateSystemAccount(cluster)
	})
	if err != nil {
		return err
	}

	return cd.deployAgent(cluster)
}

func (cd *clusterDeploy) deployAgent(cluster *v3.Cluster) error {
	desired := cluster.Spec.DesiredAgentImage
	if desired == "" || desired == "fixed" {
		desired = settings.AgentImage.Get()
	}

	if cluster.Status.AgentImage == desired {
		return nil
	}

	kubeConfig, err := cd.getKubeConfig(cluster)
	if err != nil {
		return err
	}

	_, err = v3.ClusterConditionAgentDeployed.Do(cluster, func() (runtime.Object, error) {
		yaml, err := cd.getYAML(cluster, desired)
		if err != nil {
			return cluster, err
		}

		output, err := kubectl.Apply(yaml, kubeConfig)
		if err != nil {
			return cluster, types.NewErrors(err, errors.New(string(output)))
		}
		v3.ClusterConditionAgentDeployed.Message(cluster, string(output))
		return cluster, nil
	})
	if err != nil {
		return err
	}

	if err := cd.deployRKE(kubeConfig, cluster); err != nil {
		return err
	}

	cluster.Status.AgentImage = desired
	if cluster.Spec.DesiredAgentImage == "fixed" {
		cluster.Spec.DesiredAgentImage = desired
	}

	return nil
}

func (cd *clusterDeploy) getKubeConfig(cluster *v3.Cluster) (*clientcmdapi.Config, error) {
	user, err := cd.systemAccountManager.GetSystemUser(cluster)
	if err != nil {
		return nil, err
	}

	token, err := cd.userManager.EnsureToken("agent-"+user.Name, "token for agent deployment", user.Name)
	if err != nil {
		return nil, err
	}

	return cd.clusterManager.KubeConfig(cluster.Name, token), nil
}

func (cd *clusterDeploy) getYAML(cluster *v3.Cluster, agentImage string) ([]byte, error) {
	token, err := cd.systemAccountManager.GetOrCreateSystemClusterToken(cluster.Name)
	if err != nil {
		return nil, err
	}

	url := settings.ServerURL.Get()
	if url == "" {
		return nil, fmt.Errorf("waiting for server-url setting to be set")
	}

	buf := &bytes.Buffer{}
	err = systemtemplate.SystemTemplate(buf, agentImage, token, url)

	return buf.Bytes(), err
}

func (cd *clusterDeploy) deployRKE(kubeConfig *clientcmdapi.Config, c *v3.Cluster) error {
	if c.Status.Driver != v3.ClusterDriverRKE {
		return nil
	}

	_, err := v3.ClusterConditionAddonDeploy.DoUntilTrue(c, func() (runtime.Object, error) {
		return c, cd.doDeployRKE(kubeConfig, c)
	})

	return err
}

func (cd *clusterDeploy) doDeployRKE(kubeConfig *clientcmdapi.Config, c *v3.Cluster) error {
	spec, err := cd.builder.GetSpec(c, false)
	if err != nil {
		return err
	}

	bundle, err := cd.builder.GetOrGenerateCerts(c)
	if err != nil {
		return err
	}

	tmp, err := ioutil.TempDir(tempDir, "rke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	tmpFile := path.Join(tmp, "kube_config_cluster.yml")
	if err := clientcmd.WriteToFile(*kubeConfig, tmpFile); err != nil {
		return err
	}

	if err := cluster.ApplyAuthzResources(context.Background(), *spec.RancherKubernetesEngineConfig, "", tmpFile, nil); err != nil {
		return err
	}

	return cluster.ConfigureCluster(context.Background(),
		*spec.RancherKubernetesEngineConfig,
		bundle.Certs(),
		"",
		tmpFile,
		nil,
		true)
}
