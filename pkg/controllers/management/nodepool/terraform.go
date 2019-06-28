package nodepool

import (
	"strconv"
	"strings"

	"github.com/rancher/norman/types/values"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/rancher/rancher/pkg/api/customization/clusterregistrationtokens"
	tfv1Types "github.com/rancher/terraform-controller/pkg/apis/terraformcontroller.cattle.io/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	terraformControllerNamespace = "terraform-controller"
	cattleCreatorIDAnnotationKey = "field.cattle.io/creatorId"
)

func (c *Controller) createTerraformObjects(nodePool *v3.NodePool) (*v3.NodePool, error) {

	cluster, err := c.Cluster.Get(nodePool.Namespace, metav1.GetOptions{})
	if err != nil {
		return nodePool, err
	}

	ntf, err := c.NodeTemplateTerraform.GetNamespaced(cluster.Annotations[cattleCreatorIDAnnotationKey], nodePool.Spec.NodeTemplateTerraformName, metav1.GetOptions{})
	if err != nil {
		return nodePool, err
	}

	var module *tfv1Types.Module
	if ntf.Spec.ModuleName != "" && ntf.Spec.Git.URL != "" {
		module, err = c.createTerraformModule(ntf.Spec.ModuleName, &ntf.Spec.Git)
		if err != nil {
			return nodePool, err
		}
	}

	var cc *corev1.Secret
	if ntf.Spec.CloudCredentialName != "" {
		ccName := strings.Split(ntf.Spec.CloudCredentialName, ":")
		cc, err = c.CloudCredential.Get(ccName[1], metav1.GetOptions{})
		if err != nil {
			return nodePool, err
		}

		cc, err = c.createTerraformSecret(ccName[1], cc)
		if err != nil {
			return nodePool, err
		}
	}

	logrus.Info(ntf)
	config, err := c.createTerraformConfig(nodePool, ntf)
	if err != nil {
		return nodePool, err
	}

	state, err := c.createTerraformState(nodePool, ntf, module, cc, config)
	if err != nil {
		return nodePool, err
	}

	logrus.Info(state)

	return nodePool, nil
}

func (c *Controller) createTerraformState(nodePool *v3.NodePool, terraform *v3.NodeTemplateTerraform, module *tfv1Types.Module, cc *corev1.Secret, config *corev1.ConfigMap) (*tfv1Types.State, error) {

	state, err := c.TerraformState.GetNamespaced(terraformControllerNamespace, nodePool.Name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	if state.Name != "" {
		return state, nil
	}

	state = &tfv1Types.State{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodePool.Name,
			Namespace: terraformControllerNamespace,
		},
		Spec: tfv1Types.StateSpec{
			ModuleName:      module.Name,
			AutoConfirm:     true,
			DestroyOnDelete: true,
			Variables: tfv1Types.Variables{
				ConfigNames: []string{
					config.Name,
				},
				SecretNames: []string{
					cc.Name,
				},
			},
		},
	}

	return c.TerraformState.Create(state)
}

func (c *Controller) createTerraformModule(name string, git *tfv1Types.GitLocation) (*tfv1Types.Module, error) {
	module, err := c.TerraformModule.GetNamespaced(terraformControllerNamespace, name, metav1.GetOptions{})
	if err != nil {
		return module, err
	}
	if module != nil {
		return module, nil
	}

	gitCopy := git.DeepCopy()
	module = &tfv1Types.Module{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: terraformControllerNamespace,
		},
		Spec: tfv1Types.ModuleSpec{
			ModuleContent: tfv1Types.ModuleContent{
				Git: *gitCopy,
			},
		},
	}

	return c.TerraformModule.Create(module)
}

func (c *Controller) createTerraformSecret(name string, cc *corev1.Secret) (*corev1.Secret, error) {
	secret, err := c.TerraformSecrets.GetNamespaced(terraformControllerNamespace, name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return secret, err
	}

	if secret.Name != "" {
		return secret, nil
	}

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: terraformControllerNamespace,
		},
		Data: cleanCloudCredentialData(cc.Data),
	}

	return c.TerraformSecrets.Create(secret)
}

func (c *Controller) createTerraformConfig(nodePool *v3.NodePool, ntf *v3.NodeTemplateTerraform) (*corev1.ConfigMap, error) {
	cm, err := c.TerraformConfigMaps.GetNamespaced(terraformControllerNamespace, nodePool.Name, metav1.GetOptions{})

	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
	}

	if cm.Name != "" {
		return cm, nil
	}

	rawTemplate, err := c.nodeTemplateTerraformGC.GetNamespaced(ntf.Namespace, ntf.Name, metav1.GetOptions{})
	data := rawTemplate.(*unstructured.Unstructured).Object
	rawConfig, _ := values.GetValue(data, ntf.Spec.Driver+"Config")
	config, _ := rawConfig.(map[string]interface{})

	token, err := c.Manager.GetOrCreateSystemClusterToken(nodePool.Namespace)
	if err != nil {
		return nil, err
	}

	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodePool.Name,
			Namespace: terraformControllerNamespace,
		},
		Data: map[string]string{
			"nodePool":           nodePool.Name,
			"desired_capacity":   strconv.Itoa(nodePool.Spec.Quantity),
			"roles":              c.generateNodeCommandRoleFlags(nodePool),
			"labels":             "--label pool=" + nodePool.Name,
			"nodeCommand":        clusterregistrationtokens.NodeCommand(token),
			"vpcId":              config["vpcId"].(string),
			"iamInstanceProfile": config["iamInstanceProfile"].(string),
			"keypairName":        config["keypairName"].(string),
			"region":             config["region"].(string),
			"securityGroup":      config["securityGroup"].(string),
			"subnetId":           config["subnetId"].(string),
		},
	}

	return c.TerraformConfigMaps.Create(cm)
}

func cleanCloudCredentialData(data map[string][]byte) map[string][]byte {
	cleaned := make(map[string][]byte)
	for key, value := range data {
		splitKey := strings.Split(key, "-")
		if len(splitKey) == 2 && strings.HasSuffix(splitKey[0], "Config") {
			cleaned[splitKey[1]] = value
		}
	}
	return cleaned
}
