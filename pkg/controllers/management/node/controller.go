package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/encryptedstore"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/taints"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"github.com/rancher/rancher/pkg/user"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	amazonec2                       = "amazonec2"
	userNodeRemoveCleanupAnnotation = "cleanup.cattle.io/user-node-remove"
	userNodeRemoveFinalizerPrefix   = "clusterscoped.controller.cattle.io/user-node-remove_"
)

// SchemaToDriverFields maps Schema field => driver field
// The opposite of this lives in pkg/controllers/management/drivers/nodedriver/machine_driver.go
var SchemaToDriverFields = map[string]map[string]string{
	"aliyunecs":     {"sshKeyContents": "sshKeypath"},
	"amazonec2":     {"sshKeyContents": "sshKeypath", "userdata": "userdata"},
	"azure":         {"customData": "customData"},
	"digitalocean":  {"sshKeyContents": "sshKeyPath", "userdata": "userdata"},
	"exoscale":      {"sshKey": "sshKey", "userdata": "userdata"},
	"openstack":     {"cacert": "cacert", "privateKeyFile": "privateKeyFile", "userDataFile": "userDataFile"},
	"otc":           {"privateKeyFile": "privateKeyFile"},
	"packet":        {"userdata": "userdata"},
	"pod":           {"userdata": "userdata"},
	"vmwarevsphere": {"cloudConfig": "cloud-config"},
	"google":        {"authEncodedJson": "authEncodedJson", "userdata": "userdata"},
}

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	secretStore, err := nodeconfig.NewStore(management.Core.Namespaces(""), management.Core)
	if err != nil {
		logrus.Fatal(err)
	}

	nodeClient := management.Management.Nodes("")

	nodeLifecycle := &Lifecycle{
		ctx:                       ctx,
		systemAccountManager:      systemaccount.NewManager(management),
		secretStore:               secretStore,
		nodeClient:                nodeClient,
		nodeTemplateClient:        management.Management.NodeTemplates(""),
		nodePoolLister:            management.Management.NodePools("").Controller().Lister(),
		nodePoolController:        management.Management.NodePools("").Controller(),
		nodeTemplateGenericClient: management.Management.NodeTemplates("").ObjectClient().UnstructuredClient(),
		configMapGetter:           management.K8sClient.CoreV1(),
		clusterLister:             management.Management.Clusters("").Controller().Lister(),
		schemaLister:              management.Management.DynamicSchemas("").Controller().Lister(),
		secretLister:              management.Core.Secrets("").Controller().Lister(),
		userManager:               management.UserManager,
		systemTokens:              management.SystemTokens,
		clusterManager:            clusterManager,
		devMode:                   os.Getenv("CATTLE_DEV_MODE") != "",
	}

	nodeClient.AddHandler(ctx, "node-controller-sync", nodeLifecycle.sync)
}

type Lifecycle struct {
	ctx                       context.Context
	systemAccountManager      *systemaccount.Manager
	secretStore               *encryptedstore.GenericEncryptedStore
	nodeTemplateGenericClient objectclient.GenericClient
	nodeClient                v3.NodeInterface
	nodeTemplateClient        v3.NodeTemplateInterface
	nodePoolLister            v3.NodePoolLister
	nodePoolController        v3.NodePoolController
	configMapGetter           typedv1.ConfigMapsGetter
	clusterLister             v3.ClusterLister
	schemaLister              v3.DynamicSchemaLister
	secretLister              corev1.SecretLister
	userManager               user.Manager
	systemTokens              systemtokens.Interface
	clusterManager            *clustermanager.Manager
	devMode                   bool
}

func (m *Lifecycle) setupCustom(obj *apimgmtv3.Node) {
	obj.Status.NodeConfig = &rketypes.RKEConfigNode{
		NodeName:         obj.Namespace + ":" + obj.Name,
		HostnameOverride: obj.Spec.RequestedHostname,
		Address:          obj.Spec.CustomConfig.Address,
		InternalAddress:  obj.Spec.CustomConfig.InternalAddress,
		User:             obj.Spec.CustomConfig.User,
		DockerSocket:     obj.Spec.CustomConfig.DockerSocket,
		SSHKey:           obj.Spec.CustomConfig.SSHKey,
		Labels:           obj.Spec.CustomConfig.Label,
		Port:             "22",
		Role:             roles(obj),
		Taints:           taints.GetRKETaintsFromStrings(obj.Spec.CustomConfig.Taints),
	}

	if obj.Status.NodeConfig.User == "" {
		obj.Status.NodeConfig.User = "root"
	}

	obj.Status.InternalNodeStatus.Addresses = []v1.NodeAddress{
		{
			Type:    v1.NodeInternalIP,
			Address: obj.Status.NodeConfig.Address,
		},
	}
}

func isCustom(obj *apimgmtv3.Node) bool {
	return obj.Spec.CustomConfig != nil && obj.Spec.CustomConfig.Address != ""
}

func (m *Lifecycle) setWaiting(node *apimgmtv3.Node) {
	apimgmtv3.NodeConditionRegistered.IsUnknown(node)
	apimgmtv3.NodeConditionRegistered.Message(node, "waiting to register with Kubernetes")
}

func (m *Lifecycle) Create(obj *apimgmtv3.Node) (runtime.Object, error) {
	if isCustom(obj) {
		m.setupCustom(obj)
		newObj, err := apimgmtv3.NodeConditionInitialized.Once(obj, func() (runtime.Object, error) {
			if err := validateCustomHost(obj); err != nil {
				return obj, err
			}
			m.setWaiting(obj)
			return obj, nil
		})
		return newObj.(*apimgmtv3.Node), err
	}

	if obj.Spec.NodeTemplateName == "" {
		return obj, nil
	}

	newObj, err := apimgmtv3.NodeConditionInitialized.Once(obj, func() (runtime.Object, error) {
		logrus.Debugf("[node-controller] Called apimgmtv3.NodeConditionInitialized.Once for [%s] in namespace [%s]", obj.Name, obj.Namespace)
		// Ensure jail is created first, else the function `NewNodeConfig` will create the full jail path (including parent jail directory) and CreateJail will remove the directory as it does not contain a done file
		if !m.devMode {
			err := jailer.CreateJail(obj.Namespace)
			if err != nil {
				return nil, errors.WithMessage(err, "node create jail error")
			}
		}

		nodeConfig, err := nodeconfig.NewNodeConfig(m.secretStore, obj)
		if err != nil {
			return obj, errors.WithMessagef(err, "failed to create node driver config for node [%v]", obj.Name)
		}

		defer nodeConfig.Cleanup()

		err = m.refreshNodeConfig(nodeConfig, obj)
		if err != nil {
			return nil, errors.WithMessagef(err, "unable to create config for node %v", obj.Name)
		}

		template, err := m.getNodeTemplate(obj.Spec.NodeTemplateName)
		if err != nil {
			return obj, err
		}
		obj.Status.NodeTemplateSpec = &template.Spec
		if obj.Spec.RequestedHostname == "" {
			obj.Spec.RequestedHostname = obj.Name
		}

		if obj.Status.NodeTemplateSpec.EngineInstallURL == "" {
			obj.Status.NodeTemplateSpec.EngineInstallURL = settings.EngineInstallURL.Get()
		}

		return obj, nil
	})

	return newObj.(*apimgmtv3.Node), err
}

func (m *Lifecycle) getNodeTemplate(nodeTemplateName string) (*apimgmtv3.NodeTemplate, error) {
	ns, n := ref.Parse(nodeTemplateName)
	logrus.Debugf("[node-controller] getNodeTemplate parsed [%s] to ns: [%s] and n: [%s]", nodeTemplateName, ns, n)
	return m.nodeTemplateClient.GetNamespaced(ns, n, metav1.GetOptions{})
}

func (m *Lifecycle) sync(_ string, machine *apimgmtv3.Node) (runtime.Object, error) {
	if machine == nil {
		return nil, nil
	}

	if machine.Annotations[userNodeRemoveCleanupAnnotation] != "true" {
		machine = m.userNodeRemoveCleanup(machine)
	}

	return m.nodeClient.Update(machine)
}

func (m *Lifecycle) refreshNodeConfig(nc *nodeconfig.NodeConfig, obj *apimgmtv3.Node) error {
	template, err := m.getNodeTemplate(obj.Spec.NodeTemplateName)
	if err != nil {
		return err
	}

	rawTemplate, err := m.nodeTemplateGenericClient.GetNamespaced(template.Namespace, template.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	data := rawTemplate.(*unstructured.Unstructured).Object
	rawConfig, ok := values.GetValue(data, template.Spec.Driver+"Config")
	if !ok {
		return fmt.Errorf("refreshNodeConfig: node config not specified for node %v", obj.Name)
	}

	if err := m.updateRawConfigFromCredential(data, rawConfig, template); err != nil {
		logrus.Debugf("[node-controller] refreshNodeConfig: error calling updateRawConfigFromCredential for [%v]: %v", obj.Name, err)
		return err
	}

	var update bool

	if template.Spec.Driver == amazonec2 {
		setEc2ClusterIDTag(rawConfig, obj.Namespace)
		logrus.Debug("[node-controller] refreshNodeConfig: Updating amazonec2 machine config")
		// TODO: Update to not be amazon specific, this needs to be moved to the driver
		update, err = nc.UpdateAmazonAuth(rawConfig)
		if err != nil {
			return err
		}
	}

	bytes, err := json.Marshal(rawConfig)
	if err != nil {
		return errors.Wrap(err, "failed to marshal node driver config")
	}

	newConfig := string(bytes)

	currentConfig, err := nc.DriverConfig()
	if err != nil {
		return err
	}

	if currentConfig != newConfig || update {
		err = nc.SetDriverConfig(string(bytes))
		if err != nil {
			return err
		}

		return nc.Save()
	}

	return nil
}

func validateCustomHost(obj *apimgmtv3.Node) error {
	if obj.Spec.Imported {
		return nil
	}

	customConfig := obj.Spec.CustomConfig
	signer, err := ssh.ParsePrivateKey([]byte(customConfig.SSHKey))
	if err != nil {
		return errors.Wrapf(err, "sshKey format is invalid")
	}
	config := &ssh.ClientConfig{
		User: customConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := ssh.Dial("tcp", customConfig.Address+":22", config)
	if err != nil {
		return errors.Wrapf(err, "Failed to validate ssh connection to address [%s]", customConfig.Address)
	}
	defer conn.Close()
	return nil
}

func roles(node *apimgmtv3.Node) []string {
	var roles []string
	if node.Spec.Etcd {
		roles = append(roles, "etcd")
	}
	if node.Spec.ControlPlane {
		roles = append(roles, "controlplane")
	}
	if node.Spec.Worker {
		roles = append(roles, "worker")
	}
	if len(roles) == 0 {
		return []string{"worker"}
	}
	return roles
}

func (m *Lifecycle) setCredFields(data interface{}, fields map[string]apimgmtv3.Field, credID string) error {
	splitID := strings.Split(credID, ":")
	if len(splitID) != 2 {
		return fmt.Errorf("invalid credential id %s", credID)
	}
	cred, err := m.secretLister.Get(namespace.GlobalNamespace, splitID[1])
	if err != nil {
		return err
	}
	if ans := convert.ToMapInterface(data); len(ans) > 0 {
		for key, val := range cred.Data {
			splitKey := strings.Split(key, "-")
			if len(splitKey) == 2 && strings.HasSuffix(splitKey[0], "Config") {
				if _, ok := fields[splitKey[1]]; ok {
					ans[splitKey[1]] = string(val)
				}
			}
		}
	}
	return nil
}

func (m *Lifecycle) updateRawConfigFromCredential(data map[string]interface{}, rawConfig interface{}, template *apimgmtv3.NodeTemplate) error {
	credID := convert.ToString(values.GetValueN(data, "spec", "cloudCredentialName"))
	if credID != "" {
		existingSchema, err := m.schemaLister.Get("", template.Spec.Driver+"config")
		if err != nil {
			return err
		}
		logrus.Debugf("[node-controller] setCredFields for credentialName %s", credID)
		err = m.setCredFields(rawConfig, existingSchema.Spec.ResourceFields, credID)
		if err != nil {
			return errors.Wrap(err, "failed to set credential fields")
		}
	}
	return nil
}
