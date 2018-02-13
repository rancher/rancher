package node

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	defaultEngineInstallURL = "https://releases.rancher.com/install-docker/17.03.2.sh"
)

func Register(management *config.ManagementContext) {
	secretStore, err := nodeconfig.NewStore(management)
	if err != nil {
		logrus.Fatal(err)
	}

	nodeClient := management.Management.Nodes("")

	nodeLifecycle := &Lifecycle{
		secretStore:               secretStore,
		nodeClient:                nodeClient,
		nodeTemplateClient:        management.Management.NodeTemplates(""),
		nodeTemplateGenericClient: management.Management.NodeTemplates("").ObjectClient().UnstructuredClient(),
		configMapGetter:           management.K8sClient.CoreV1(),
		logger:                    management.EventLogger,
		clusterLister:             management.Management.Clusters("").Controller().Lister(),
	}

	nodeClient.AddLifecycle("node-controller", nodeLifecycle)
}

type Lifecycle struct {
	secretStore               *encryptedstore.GenericEncryptedStore
	nodeTemplateGenericClient clientbase.GenericClient
	nodeClient                v3.NodeInterface
	nodeTemplateClient        v3.NodeTemplateInterface
	configMapGetter           typedv1.ConfigMapsGetter
	logger                    event.Logger
	clusterLister             v3.ClusterLister
}

func (m *Lifecycle) setupCustom(obj *v3.Node) {
	obj.Status.NodeConfig = &v3.RKEConfigNode{
		NodeName:         obj.Spec.ClusterName + ":" + obj.Name,
		HostnameOverride: obj.Spec.RequestedHostname,
		Address:          obj.Spec.CustomConfig.Address,
		InternalAddress:  obj.Spec.CustomConfig.InternalAddress,
		User:             obj.Spec.CustomConfig.User,
		DockerSocket:     obj.Spec.CustomConfig.DockerSocket,
		SSHKey:           obj.Spec.CustomConfig.SSHKey,
		Role:             obj.Spec.CustomConfig.Roles,
	}

	if obj.Status.NodeConfig.User == "" {
		obj.Status.NodeConfig.User = "root"
	}
}

func isCustom(obj *v3.Node) bool {
	return obj.Spec.CustomConfig != nil && obj.Spec.CustomConfig.Address != ""
}

func (m *Lifecycle) Create(obj *v3.Node) (*v3.Node, error) {
	if isCustom(obj) {
		m.setupCustom(obj)
		newObj, err := v3.NodeConditionInitialized.Once(obj, func() (runtime.Object, error) {
			if err := validateCustomHost(obj); err != nil {
				return obj, err
			}
			return obj, nil
		})
		return newObj.(*v3.Node), err
	}

	if obj.Spec.NodeTemplateName == "" {
		return obj, nil
	}

	newObj, err := v3.NodeConditionInitialized.Once(obj, func() (runtime.Object, error) {
		template, err := m.nodeTemplateClient.Get(obj.Spec.NodeTemplateName, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}
		obj.Status.NodeTemplateSpec = &template.Spec
		if obj.Spec.RequestedHostname == "" {
			obj.Spec.RequestedHostname = obj.Name
		}

		if obj.Status.NodeTemplateSpec.EngineInstallURL == "" {
			obj.Status.NodeTemplateSpec.EngineInstallURL = defaultEngineInstallURL
		}

		rawTemplate, err := m.nodeTemplateGenericClient.Get(obj.Spec.NodeTemplateName, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}

		rawConfig, ok := values.GetValue(rawTemplate.(*unstructured.Unstructured).Object, template.Spec.Driver+"Config")
		if !ok {
			return obj, fmt.Errorf("node config not specified")
		}

		bytes, err := json.Marshal(rawConfig)
		if err != nil {
			return obj, errors.Wrap(err, "failed to marshal node driver confg")
		}

		config, err := nodeconfig.NewNodeConfig(m.secretStore, obj)
		if err != nil {
			return obj, errors.Wrap(err, "failed to save node driver config")
		}
		defer config.Cleanup()

		config.SetDriverConfig(string(bytes))

		return obj, config.Save()
	})

	return newObj.(*v3.Node), err
}

func (m *Lifecycle) Remove(obj *v3.Node) (*v3.Node, error) {
	if obj.Status.NodeTemplateSpec == nil {
		return obj, nil
	}
	found, err := m.isNodeInAppliedSpec(obj)
	if err != nil {
		return obj, err
	}
	if found {
		return obj, fmt.Errorf("Node [%s] still not deleted from cluster spec", obj.Name)
	}
	config, err := nodeconfig.NewNodeConfig(m.secretStore, obj)
	if err != nil {
		return obj, err
	}
	if err := config.Restore(); err != nil {
		return obj, err
	}
	defer config.Remove()

	mExists, err := nodeExists(config.Dir(), obj.Spec.RequestedHostname)
	if err != nil {
		return obj, err
	}

	if mExists {
		m.logger.Infof(obj, "Removing node %s", obj.Spec.RequestedHostname)
		if err := deleteNode(config.Dir(), obj); err != nil {
			return obj, err
		}
		m.logger.Infof(obj, "Removing node %s done", obj.Spec.RequestedHostname)
	}

	return obj, nil
}

func (m *Lifecycle) provision(driverConfig, nodeDir string, obj *v3.Node) (*v3.Node, error) {
	configRawMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(driverConfig), &configRawMap); err != nil {
		return obj, errors.Wrap(err, "failed to unmarshal node config")
	}

	// Since we know this will take a long time persist so user sees status
	obj, err := m.nodeClient.Update(obj)
	if err != nil {
		return obj, err
	}

	createCommandsArgs := buildCreateCommand(obj, configRawMap)
	cmd := buildCommand(nodeDir, createCommandsArgs)
	m.logger.Infof(obj, "Provisioning node %s", obj.Spec.RequestedHostname)

	stdoutReader, stderrReader, err := startReturnOutput(cmd)
	if err != nil {
		return obj, err
	}
	defer stdoutReader.Close()
	defer stderrReader.Close()
	defer cmd.Wait()

	hostExist := false
	obj, err = m.reportStatus(stdoutReader, stderrReader, obj)
	if err != nil {
		if strings.Contains(err.Error(), "Host already exists") {
			hostExist = true
		}
		if !hostExist {
			return obj, err
		}
	}

	if err := cmd.Wait(); err != nil && !hostExist {
		return obj, err
	}

	m.logger.Infof(obj, "Provisioning node %s done", obj.Spec.RequestedHostname)
	return obj, nil
}

func (m *Lifecycle) ready(obj *v3.Node) (*v3.Node, error) {
	config, err := nodeconfig.NewNodeConfig(m.secretStore, obj)
	if err != nil {
		return obj, err
	}
	defer config.Cleanup()

	if err := config.Restore(); err != nil {
		return obj, err
	}

	driverConfig, err := config.DriverConfig()
	if err != nil {
		return nil, err
	}

	// Provision in the background so we can poll and save the config
	done := make(chan error)
	go func() {
		newObj, err := v3.NodeConditionProvisioned.Once(obj, func() (runtime.Object, error) {
			return m.provision(driverConfig, config.Dir(), obj)
		})
		obj = newObj.(*v3.Node)
		done <- err
	}()

	// Poll and save config
outer:
	for {
		select {
		case err = <-done:
			break outer
		case <-time.After(5 * time.Second):
			config.Save()
		}
	}

	newObj, saveError := v3.NodeConditionConfigSaved.Once(obj, func() (runtime.Object, error) {
		return m.saveConfig(config, config.Dir(), obj)
	})
	obj = newObj.(*v3.Node)
	if err == nil {
		return obj, saveError
	}
	return obj, err
}

func (m *Lifecycle) Updated(obj *v3.Node) (*v3.Node, error) {
	if obj.Status.NodeTemplateSpec == nil {
		return obj, nil
	}

	newObj, err := v3.NodeConditionReady.Once(obj, func() (runtime.Object, error) {
		return m.ready(obj)
	})
	obj = newObj.(*v3.Node)

	return obj, err
}

func (m *Lifecycle) saveConfig(config *nodeconfig.NodeConfig, nodeDir string, obj *v3.Node) (*v3.Node, error) {
	logrus.Infof("Generating and uploading node config %s", obj.Spec.RequestedHostname)
	if err := config.Save(); err != nil {
		return obj, err
	}

	ip, err := config.IP()
	if err != nil {
		return obj, err
	}

	interalAddress, err := config.InternalIP()
	if err != nil {
		return obj, err
	}

	sshKey, err := getSSHKey(nodeDir, obj)
	if err != nil {
		return obj, err
	}

	sshUser, err := config.SSHUser()
	if err != nil {
		return obj, err
	}

	if err := config.Save(); err != nil {
		return obj, err
	}

	obj.Status.NodeConfig = &v3.RKEConfigNode{
		NodeName:         obj.Spec.ClusterName + ":" + obj.Name,
		Address:          ip,
		InternalAddress:  interalAddress,
		User:             sshUser,
		Role:             roles(obj),
		HostnameOverride: obj.Spec.RequestedHostname,
		SSHKey:           sshKey,
	}

	if len(obj.Status.NodeConfig.Role) == 0 {
		obj.Status.NodeConfig.Role = []string{"worker"}
	}

	return obj, nil
}

func (m *Lifecycle) isNodeInAppliedSpec(node *v3.Node) (bool, error) {
	cluster, err := m.clusterLister.Get("", node.Spec.ClusterName)
	if err != nil {
		if kerror.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if cluster == nil {
		return false, nil
	}
	if cluster.DeletionTimestamp != nil {
		return false, nil
	}
	if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
		return false, nil
	}

	for _, rkeNode := range cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Nodes {
		nodeName := rkeNode.NodeName
		if len(nodeName) == 0 {
			continue
		}
		if nodeName == fmt.Sprintf("%s:%s", node.Namespace, node.Name) {
			return true, nil
		}
	}
	return false, nil
}

func validateCustomHost(obj *v3.Node) error {
	if obj.Spec.CustomConfig != nil && obj.Spec.CustomConfig.Address != "" {
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
		conn.Close()
	}
	return nil
}

func roles(node *v3.Node) []string {
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
