package machine

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/machine/store"
	machineconfig "github.com/rancher/rancher/pkg/machine/store/config"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
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
	secretStore, err := machineconfig.NewStore(management)
	if err != nil {
		logrus.Fatal(err)
	}

	machineClient := management.Management.Machines("")

	machineLifecycle := &Lifecycle{
		secretStore:                  secretStore,
		machineClient:                machineClient,
		machineTemplateClient:        management.Management.MachineTemplates(""),
		machineTemplateGenericClient: management.Management.MachineTemplates("").ObjectClient().UnstructuredClient(),
		configMapGetter:              management.K8sClient.CoreV1(),
		logger:                       management.EventLogger,
		clusterLister:                management.Management.Clusters("").Controller().Lister(),
	}

	machineClient.AddLifecycle("machine-controller", machineLifecycle)
}

type Lifecycle struct {
	secretStore                  *store.GenericEncryptedStore
	machineTemplateGenericClient *clientbase.ObjectClient
	machineClient                v3.MachineInterface
	machineTemplateClient        v3.MachineTemplateInterface
	configMapGetter              typedv1.ConfigMapsGetter
	logger                       event.Logger
	clusterLister                v3.ClusterLister
}

func (m *Lifecycle) setupCustom(obj *v3.Machine) {
	obj.Status.NodeConfig = &v3.RKEConfigNode{
		MachineName:      obj.Spec.ClusterName + ":" + obj.Name,
		Role:             obj.Spec.Role,
		HostnameOverride: obj.Spec.RequestedHostname,
		Address:          obj.Spec.CustomConfig.Address,
		InternalAddress:  obj.Spec.CustomConfig.InternalAddress,
		User:             obj.Spec.CustomConfig.User,
		DockerSocket:     obj.Spec.CustomConfig.DockerSocket,
		SSHKey:           obj.Spec.CustomConfig.SSHKey,
	}

	obj.Status.SSHUser = obj.Status.NodeConfig.User
	if obj.Status.SSHUser == "" {
		obj.Status.SSHUser = "root"
	}
}

func isCustom(obj *v3.Machine) bool {
	return obj.Spec.CustomConfig != nil && obj.Spec.CustomConfig.Address != ""
}

func (m *Lifecycle) Create(obj *v3.Machine) (*v3.Machine, error) {
	if isCustom(obj) {
		m.setupCustom(obj)
		v3.MachineConditionReady.True(obj)
		return obj, nil
	}

	if obj.Spec.MachineTemplateName == "" {
		return obj, nil
	}

	newObj, err := v3.MachineConditionInitialized.Once(obj, func() (runtime.Object, error) {
		template, err := m.machineTemplateClient.Get(obj.Spec.MachineTemplateName, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}
		obj.Status.MachineTemplateSpec = &template.Spec
		if obj.Spec.RequestedHostname == "" {
			obj.Spec.RequestedHostname = obj.Name
		}

		if obj.Status.MachineTemplateSpec.EngineInstallURL == "" {
			obj.Status.MachineTemplateSpec.EngineInstallURL = defaultEngineInstallURL
		}

		rawTemplate, err := m.machineTemplateGenericClient.Get(obj.Spec.MachineTemplateName, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}

		rawConfig, ok := values.GetValue(rawTemplate.(*unstructured.Unstructured).Object, template.Spec.Driver+"Config")
		if !ok {
			return obj, fmt.Errorf("machine config not specified")
		}

		sshUser, ok := convert.ToMapInterface(rawConfig)["sshUser"]
		if ok {
			obj.Status.SSHUser = convert.ToString(sshUser)
		}

		if obj.Status.SSHUser == "" {
			obj.Status.SSHUser = "root"
		}

		bytes, err := json.Marshal(rawConfig)
		if err != nil {
			return obj, errors.Wrap(err, "failed to marshal machine driver confg")
		}

		obj.Status.MachineDriverConfig = string(bytes)
		return obj, nil
	})

	return newObj.(*v3.Machine), err
}

func (m *Lifecycle) Remove(obj *v3.Machine) (*v3.Machine, error) {
	if obj.Status.MachineTemplateSpec == nil {
		return obj, nil
	}
	found, err := m.isNodeInAppliedSpec(obj)
	if err != nil {
		return obj, err
	}
	if found {
		return obj, fmt.Errorf("Machine [%s] still not deleted from cluster spec", obj.Name)
	}
	config, err := machineconfig.NewMachineConfig(m.secretStore, obj)
	if err != nil {
		return obj, err
	}
	if err := config.Restore(); err != nil {
		return obj, err
	}
	defer config.Remove()

	mExists, err := machineExists(config.Dir(), obj.Spec.RequestedHostname)
	if err != nil {
		return obj, err
	}

	if mExists {
		m.logger.Infof(obj, "Removing machine %s", obj.Spec.RequestedHostname)
		if err := deleteMachine(config.Dir(), obj); err != nil {
			return obj, err
		}
		m.logger.Infof(obj, "Removing machine %s done", obj.Spec.RequestedHostname)
	}

	return obj, nil
}

func (m *Lifecycle) provision(machineDir string, obj *v3.Machine) (*v3.Machine, error) {
	configRawMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(obj.Status.MachineDriverConfig), &configRawMap); err != nil {
		return obj, errors.Wrap(err, "failed to unmarshal machine config")
	}

	// Since we know this will take a long time persist so user sees status
	obj, err := m.machineClient.Update(obj)
	if err != nil {
		return obj, err
	}

	createCommandsArgs := buildCreateCommand(obj, configRawMap)
	cmd := buildCommand(machineDir, createCommandsArgs)
	m.logger.Infof(obj, "Provisioning machine %s", obj.Spec.RequestedHostname)

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

	m.logger.Infof(obj, "Provisioning machine %s done", obj.Spec.RequestedHostname)
	return obj, nil
}

func (m *Lifecycle) ready(obj *v3.Machine) (*v3.Machine, error) {
	config, err := machineconfig.NewMachineConfig(m.secretStore, obj)
	if err != nil {
		return obj, err
	}
	defer config.Cleanup()

	if err := config.Restore(); err != nil {
		return obj, err
	}

	// Provision in the background so we can poll and save the config
	done := make(chan error)
	go func() {
		newObj, err := v3.MachineConditionProvisioned.Once(obj, func() (runtime.Object, error) {
			return m.provision(config.Dir(), obj)
		})
		obj = newObj.(*v3.Machine)
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

	newObj, saveError := v3.MachineConditionConfigSaved.Once(obj, func() (runtime.Object, error) {
		return m.saveConfig(config, config.Dir(), obj)
	})
	obj = newObj.(*v3.Machine)
	if err == nil {
		return obj, saveError
	}
	return obj, err
}

func (m *Lifecycle) Updated(obj *v3.Machine) (*v3.Machine, error) {
	if obj.Status.MachineTemplateSpec == nil {
		return obj, nil
	}

	newObj, err := v3.MachineConditionReady.Once(obj, func() (runtime.Object, error) {
		return m.ready(obj)
	})
	obj = newObj.(*v3.Machine)

	return obj, err
}

func (m *Lifecycle) saveConfig(config *machineconfig.MachineConfig, machineDir string, obj *v3.Machine) (*v3.Machine, error) {
	logrus.Infof("Generating and uploading machine config %s", obj.Spec.RequestedHostname)
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

	sshKey, err := getSSHKey(machineDir, obj)
	if err != nil {
		return obj, err
	}

	if err := config.Save(); err != nil {
		return obj, err
	}

	obj.Status.NodeConfig = &v3.RKEConfigNode{
		MachineName:      obj.Spec.ClusterName + ":" + obj.Name,
		Address:          ip,
		InternalAddress:  interalAddress,
		User:             obj.Status.SSHUser,
		Role:             obj.Spec.Role,
		HostnameOverride: obj.Spec.RequestedHostname,
		SSHKey:           sshKey,
	}

	if len(obj.Status.NodeConfig.Role) == 0 {
		obj.Status.NodeConfig.Role = []string{"worker"}
	}

	return obj, nil
}

func (m *Lifecycle) isNodeInAppliedSpec(machine *v3.Machine) (bool, error) {
	cluster, err := m.clusterLister.Get("", machine.Spec.ClusterName)
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

	for _, node := range cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Nodes {
		machineName := node.MachineName
		if len(machineName) == 0 {
			continue
		}
		if machineName == fmt.Sprintf("%s:%s", machine.Namespace, machine.Name) {
			return true, nil
		}
	}
	return false, nil
}
