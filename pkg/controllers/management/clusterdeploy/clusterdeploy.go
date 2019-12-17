package clusterdeploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/rancher/norman/types"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/systemtemplate"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const AgentForceDeployAnn = "io.cattle.agent.force.deploy"

var (
	nodeAgentImages    map[k8stypes.UID]string
	nodeMutex          sync.RWMutex
	clusterAgentImages map[k8stypes.UID]string
	clusterMutex       sync.RWMutex
)

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	nodeAgentImages = make(map[k8stypes.UID]string)
	clusterAgentImages = make(map[k8stypes.UID]string)
	c := &clusterDeploy{
		systemAccountManager: systemaccount.NewManager(management),
		userManager:          management.UserManager,
		clusters:             management.Management.Clusters(""),
		nodeLister:           management.Management.Nodes("").Controller().Lister(),
		clusterManager:       clusterManager,
	}

	management.Management.Clusters("").AddHandler(ctx, "cluster-deploy", c.sync)
}

type clusterDeploy struct {
	systemAccountManager *systemaccount.Manager
	userManager          user.Manager
	clusters             v3.ClusterInterface
	clusterManager       *clustermanager.Manager
	nodeLister           v3.NodeLister
}

func (cd *clusterDeploy) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	var (
		err, updateErr error
	)

	if cluster == nil || cluster.DeletionTimestamp != nil {
		// remove the system account user created for this cluster
		if err := cd.systemAccountManager.RemoveSystemAccount(key); err != nil {
			return nil, err
		}
		return nil, nil
	}

	original := cluster
	cluster = original.DeepCopy()

	if cluster.Status.Driver == v3.ClusterDriverRKE {
		if cluster.Spec.LocalClusterAuthEndpoint.Enabled {
			cluster.Spec.RancherKubernetesEngineConfig.Authentication.Strategy = "x509|webhook"
		} else {
			cluster.Spec.RancherKubernetesEngineConfig.Authentication.Strategy = "x509"
		}
	}

	err = cd.doSync(cluster)
	if cluster != nil && !reflect.DeepEqual(cluster, original) {
		_, updateErr = cd.clusters.Update(cluster)
	}

	if err != nil {
		return nil, err
	}
	return nil, updateErr
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

	err = cd.validateAgent(cluster)
	if err != nil {
		return err
	}

	return cd.setNetworkPolicyAnn(cluster)
}

func redeployAgent(cluster *v3.Cluster, desiredAgent, desiredAuth string) bool {
	forceDeploy := cluster.Annotations[AgentForceDeployAnn] == "true"
	imageChange := cluster.Status.AgentImage != desiredAgent || cluster.Status.AuthImage != desiredAuth
	repoChange := false
	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
			desiredRepo := util.GetPrivateRepo(cluster)
			var appliedRepo *v3.PrivateRegistry
			if len(cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries) > 0 {
				appliedRepo = &cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries[0]
			}
			if desiredRepo != nil && appliedRepo != nil && !reflect.DeepEqual(desiredRepo, appliedRepo) {
				repoChange = true
			}
			if (desiredRepo == nil && appliedRepo != nil) || (desiredRepo != nil && appliedRepo == nil) {
				repoChange = true
			}
		}
	}

	if forceDeploy || imageChange || repoChange {
		logrus.Infof("Redeploy Rancher Agents is needed for %s: forceDeploy=%v, agent/auth image changed=%v,"+
			" private repo changed=%v", cluster.Name, forceDeploy, imageChange, repoChange)
		return true
	}
	return false
}

func (cd *clusterDeploy) deployAgent(cluster *v3.Cluster) error {
	desiredAgent := cluster.Spec.DesiredAgentImage
	if desiredAgent == "" || desiredAgent == "fixed" {
		desiredAgent = image.ResolveWithCluster(settings.AgentImage.Get(), cluster)
	}

	var desiredAuth string
	if cluster.Spec.LocalClusterAuthEndpoint.Enabled {
		desiredAuth = cluster.Spec.DesiredAuthImage
		if desiredAuth == "" || desiredAuth == "fixed" {
			desiredAuth = image.ResolveWithCluster(settings.AuthImage.Get(), cluster)
		}
	}

	if !redeployAgent(cluster, desiredAgent, desiredAuth) {
		return nil
	}

	kubeConfig, err := cd.getKubeConfig(cluster)
	if err != nil {
		return err
	}

	_, err = v3.ClusterConditionAgentDeployed.Do(cluster, func() (runtime.Object, error) {
		yaml, err := cd.getYAML(cluster, desiredAgent, desiredAuth)
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
		if !cluster.Spec.LocalClusterAuthEndpoint.Enabled && cluster.Status.AppliedSpec.LocalClusterAuthEndpoint.Enabled && cluster.Status.AuthImage != "" {
			output, err = kubectl.Delete([]byte(systemtemplate.AuthDaemonSet), kubeConfig)
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
		cluster.Status.AgentImage = desiredAgent
		if cluster.Spec.DesiredAgentImage == "fixed" {
			cluster.Spec.DesiredAgentImage = desiredAgent
		}
		cluster.Status.AuthImage = desiredAuth
		if cluster.Spec.DesiredAuthImage == "fixed" {
			cluster.Spec.DesiredAuthImage = desiredAuth
		}
		if cluster.Annotations[AgentForceDeployAnn] == "true" {
			cluster.Annotations[AgentForceDeployAnn] = "false"
		}
	}

	return err
}

func (cd *clusterDeploy) validateAgent(cluster *v3.Cluster) error {
	if cluster.Status.AgentImage == "" {
		//if empty we're waiting for something to deploy, valid until proven otherwise.
		return nil
	}

	//check if the agent downstream is the same as the one locally in the crd
	var na, ca string
	var ok bool
	var err error
	if na, ok = nodeAgentImages[cluster.UID]; !ok {
		na, err = cd.getNodeAgentImage(cluster)
		if err != nil {
			return err
		}
		nodeMutex.Lock()
		nodeAgentImages[cluster.UID] = na
		nodeMutex.Unlock()
	}

	if ca, ok = clusterAgentImages[cluster.UID]; !ok {
		ca, err = cd.getClusterAgentImage(cluster)
		if err != nil {
			return err
		}
		clusterMutex.Lock()
		clusterAgentImages[cluster.UID] = ca
		clusterMutex.Unlock()
	}

	if cluster.Status.AgentImageOverride != "" {
		if cluster.Status.AgentImageOverride == cluster.Spec.DesiredAgentImage {
			return nil
		}
		if cluster.Status.AgentImageOverride == cluster.Status.AgentImage {
			return nil
		}
		clearAgentCaches(cluster.UID)
		cluster.Status.AgentImage = ""
		cluster.Spec.DesiredAgentImage = cluster.Status.AgentImageOverride
		return fmt.Errorf("Rancher Agent Override specificied, setting desired image: %s",
			cluster.Status.AgentImageOverride)
	}

	if cluster.Status.AgentImage != na || cluster.Status.AgentImage != ca {
		//downstream doesn't match and we should refresh
		err = fmt.Errorf("Local agent image does not match downstream: %s locally, %s agent, %s cluster",
			cluster.Status.AgentImage,
			nodeAgentImages[cluster.UID],
			clusterAgentImages[cluster.UID])
		clearAgentCaches(cluster.UID)
		cluster.Status.AgentImage = ""
		cluster.Spec.DesiredAgentImage = ""
		return err
	}

	return nil
}

func (cd *clusterDeploy) setNetworkPolicyAnn(cluster *v3.Cluster) error {
	if cluster.Spec.EnableNetworkPolicy != nil {
		return nil
	}
	// set current state for upgraded canal clusters
	if cluster.Spec.RancherKubernetesEngineConfig != nil &&
		cluster.Spec.RancherKubernetesEngineConfig.Network.Plugin == "canal" {
		enableNetworkPolicy := true
		cluster.Spec.EnableNetworkPolicy = &enableNetworkPolicy
		cluster.Annotations["networking.management.cattle.io/enable-network-policy"] = "true"
	}
	return nil
}

func (cd *clusterDeploy) getKubeConfig(cluster *v3.Cluster) (*clientcmdapi.Config, error) {
	user, err := cd.systemAccountManager.GetSystemUser(cluster.Name)
	if err != nil {
		return nil, err
	}

	token, err := cd.userManager.EnsureToken("agent-"+user.Name, "token for agent deployment", "agent", user.Name)
	if err != nil {
		return nil, err
	}

	return cd.clusterManager.KubeConfig(cluster.Name, token), nil
}

func (cd *clusterDeploy) getYAML(cluster *v3.Cluster, agentImage, authImage string) ([]byte, error) {
	logrus.Debug("Desired agent image:", agentImage)
	logrus.Debug("Desired auth image:", authImage)

	token, err := cd.systemAccountManager.GetOrCreateSystemClusterToken(cluster.Name)
	if err != nil {
		return nil, err
	}

	url := settings.ServerURL.Get()
	if url == "" {
		return nil, fmt.Errorf("waiting for server-url setting to be set")
	}

	buf := &bytes.Buffer{}
	err = systemtemplate.SystemTemplate(buf, agentImage, authImage, token, url, cluster.Spec.WindowsPreferedCluster,
		cluster)

	return buf.Bytes(), err
}

func (cd *clusterDeploy) getClusterAgentImage(cluster *v3.Cluster) (string, error) {
	uc, err := cd.clusterManager.UserContext(cluster.Name)
	if err != nil {
		return "", err
	}

	d, err := uc.Apps.Deployments("cattle-system").Get("cattle-cluster-agent", v1.GetOptions{})
	if err != nil {
		return "", err
	}

	if len(d.Spec.Template.Spec.Containers) != 1 {
		return "", fmt.Errorf("Expected one container in cattle-cluster-agent Deployment, found %d",
			len(d.Spec.Template.Spec.Containers))
	}

	return d.Spec.Template.Spec.Containers[0].Image, nil
}

func (cd *clusterDeploy) getNodeAgentImage(cluster *v3.Cluster) (string, error) {
	uc, err := cd.clusterManager.UserContext(cluster.Name)
	if err != nil {
		return "", err
	}

	ds, err := uc.Apps.DaemonSets("cattle-system").Get("cattle-node-agent", v1.GetOptions{})
	if err != nil {
		return "", err
	}

	if len(ds.Spec.Template.Spec.Containers) != 1 {
		return "", fmt.Errorf("Expected one container in cattle-node-agent DaemonSet, found %d",
			len(ds.Spec.Template.Spec.Containers))
	}

	return ds.Spec.Template.Spec.Containers[0].Image, nil
}

func clearAgentCaches(uid k8stypes.UID) {
	nodeMutex.Lock()
	delete(nodeAgentImages, uid)
	nodeMutex.Unlock()

	clusterMutex.Lock()
	delete(clusterAgentImages, uid)
	clusterMutex.Unlock()
}
