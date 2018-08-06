package clusterdeploy

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func Register(management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	c := &clusterDeploy{
		systemAccountManager: systemaccount.NewManager(management),
		userManager:          management.UserManager,
		clusters:             management.Management.Clusters(""),
		nodeLister:           management.Management.Nodes("").Controller().Lister(),
		clusterManager:       clusterManager,
	}

	management.Management.Clusters("").AddHandler("cluster-deploy", c.sync)
}

type clusterDeploy struct {
	systemAccountManager *systemaccount.Manager
	userManager          user.Manager
	clusters             v3.ClusterInterface
	clusterManager       *clustermanager.Manager
	nodeLister           v3.NodeLister
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

	nodes, err := cd.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	_, err = v3.ClusterConditionSystemAccountCreated.DoUntilTrue(cluster, func() (runtime.Object, error) {
		return cluster, cd.systemAccountManager.CreateSystemAccount(cluster)
	})
	if err != nil {
		return err
	}
	err = cd.deployAgent(cluster)
	if err != nil {
		return err
	}

	return cd.setNetworkPolicyAnn(cluster)
}

func (cd *clusterDeploy) deployAgent(cluster *v3.Cluster) error {
	desired := cluster.Spec.DesiredAgentImage
	if desired == "" || desired == "fixed" {
		desired = image.Resolve(settings.AgentImage.Get())
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

		var output []byte
		for i := 0; i < 3; i++ {
			// This will fail almost always the first time because when we create the namespace in the file
			// it won't have privileges.  Just stupidly try 3 times
			output, err = kubectl.Apply(yaml, kubeConfig)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			return cluster, types.NewErrors(err, errors.New(string(output)))
		}
		v3.ClusterConditionAgentDeployed.Message(cluster, string(output))
		return cluster, nil
	})
	if err != nil {
		return err
	}

	if err == nil {
		cluster.Status.AgentImage = desired
		if cluster.Spec.DesiredAgentImage == "fixed" {
			cluster.Spec.DesiredAgentImage = desired
		}
	}

	return err
}

func (cd *clusterDeploy) setNetworkPolicyAnn(cluster *v3.Cluster) error {
	if cluster.Spec.EnableNetworkPolicy != nil {
		return nil
	}
	// set current state for upgraded canal clusters
	if cluster.Spec.RancherKubernetesEngineConfig != nil &&
		cluster.Spec.RancherKubernetesEngineConfig.Network.CanalNetworkProvider != nil {
		enableNetworkPolicy := true
		cluster.Spec.EnableNetworkPolicy = &enableNetworkPolicy
		cluster.Annotations["networking.management.cattle.io/enable-network-policy"] = "true"
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
