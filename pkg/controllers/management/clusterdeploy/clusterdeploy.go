package clusterdeploy

import (
	"errors"
	"fmt"

	"reflect"

	"bytes"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	clusterOwnerRole = "cluster-owner"
)

func Register(management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	c := &clusterDeploy{
		userManager:    management.UserManager,
		crtbs:          management.Management.ClusterRoleTemplateBindings(""),
		crts:           management.Management.ClusterRegistrationTokens(""),
		clusters:       management.Management.Clusters(""),
		clusterManager: clusterManager,
	}

	management.Management.Clusters("").AddHandler("cluster-deploy", c.sync)
}

type clusterDeploy struct {
	userManager    user.Manager
	crtbs          v3.ClusterRoleTemplateBindingInterface
	clusters       v3.ClusterInterface
	crts           v3.ClusterRegistrationTokenInterface
	clusterManager *clustermanager.Manager
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

	_, err := v3.ClusterConditionSystemAccountCreated.DoUntilTrue(cluster, func() (runtime.Object, error) {
		return cluster, cd.createSystemAccount(cluster)
	})
	if err != nil {
		return err
	}

	return cd.deployAgent(cluster)
}

func (cd *clusterDeploy) createSystemAccount(cluster *v3.Cluster) error {
	user, err := cd.getUser(cluster)
	if err != nil {
		return err
	}

	bindingName := user.Name + "-admin"
	_, err = cd.crtbs.GetNamespaced(cluster.Name, bindingName, v1.GetOptions{})
	if err == nil {
		return nil
	}

	_, err = cd.crtbs.Create(&v3.ClusterRoleTemplateBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      bindingName,
			Namespace: cluster.Name,
		},
		ClusterName:      cluster.Name,
		UserName:         user.Name,
		RoleTemplateName: clusterOwnerRole,
	})

	return err
}

func (cd *clusterDeploy) getUser(cluster *v3.Cluster) (*v3.User, error) {
	return cd.userManager.EnsureUser(fmt.Sprintf("system://%s", cluster.Name), "System account for Cluster "+cluster.Spec.DisplayName)
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

	if err == nil {
		cluster.Status.AgentImage = desired
		if cluster.Spec.DesiredAgentImage == "fixed" {
			cluster.Spec.DesiredAgentImage = desired
		}
	}

	return err
}

func (cd *clusterDeploy) getKubeConfig(cluster *v3.Cluster) (*clientcmdapi.Config, error) {
	user, err := cd.getUser(cluster)
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
	token, err := cd.getClusterToken(cluster)
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

func (cd *clusterDeploy) getClusterToken(cluster *v3.Cluster) (string, error) {
	token := ""

	crt, err := cd.crts.GetNamespaced(cluster.Name, "system", v1.GetOptions{})
	if errors2.IsNotFound(err) {
		token, err = randomtoken.Generate()
		if err != nil {
			return "", err
		}
		crt = &v3.ClusterRegistrationToken{
			ObjectMeta: v1.ObjectMeta{
				Name:      "system",
				Namespace: cluster.Name,
			},
			Spec: v3.ClusterRegistrationTokenSpec{
				ClusterName: cluster.Name,
			},
			Status: v3.ClusterRegistrationTokenStatus{
				Token: token,
			},
		}

		if _, err := cd.crts.Create(crt); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		token = crt.Status.Token
	}

	return token, nil
}
