package machine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	machineconfig "github.com/rancher/machine-controller/controller/machine/config"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Register(management *config.ManagementContext) {
	machineClient := management.Management.Machines("")

	machineLifecycle := &Lifecycle{
		machineClient:                machineClient,
		machineTemplateClient:        management.Management.MachineTemplates(""),
		machineTemplateGenericClient: management.Management.MachineTemplates("").ObjectClient().UnstructuredClient(),
		configMapGetter:              management.K8sClient.CoreV1(),
		logger:                       management.EventLogger,
	}

	machineClient.AddLifecycle("machine-controller", machineLifecycle)
}

type Lifecycle struct {
	machineTemplateGenericClient *clientbase.ObjectClient
	machineClient                v3.MachineInterface
	machineTemplateClient        v3.MachineTemplateInterface
	configMapGetter              typedv1.ConfigMapsGetter
	logger                       event.Logger
}

func (m *Lifecycle) Create(obj *v3.Machine) (*v3.Machine, error) {
	if obj.Spec.MachineTemplateName == "" {
		return obj, nil
	}

	newObj, err := v3.MachineConditionInitialized.Once(obj, func() (runtime.Object, error) {
		template, err := m.machineTemplateClient.Get(obj.Spec.MachineTemplateName, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}
		obj.Status.MachineTemplateSpec = &template.Spec

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

	machineDir, err := buildBaseHostDir(obj.Spec.DisplayName)
	if err != nil {
		return obj, err
	}
	logrus.Debugf("Creating machine storage directory %s", machineDir)

	config := machineconfig.NewMachineConfig(m.configMapGetter, machineDir, obj.Name)
	if err := config.Restore(); err != nil {
		return obj, err
	}
	defer config.Cleanup()

	mExists, err := machineExists(machineDir, obj.Spec.DisplayName)
	if err != nil {
		return obj, err
	}

	if mExists {
		m.logger.Infof(obj, "Removing machine %s", obj.Spec.DisplayName)
		if err := deleteMachine(machineDir, obj); err != nil {
			return nil, err
		}
		m.logger.Infof(obj, "Removing machine %s done", obj.Spec.DisplayName)
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
	m.logger.Infof(obj, "Provisioning machine %s", obj.Spec.DisplayName)

	stdoutReader, stderrReader, err := startReturnOutput(cmd)
	if err != nil {
		return obj, err
	}
	defer stdoutReader.Close()
	defer stderrReader.Close()
	defer cmd.Wait()

	hostExist := false
	if err := m.reportStatus(stdoutReader, stderrReader, obj); err != nil {
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

	m.logger.Infof(obj, "Provisioning machine %s done", obj.Spec.DisplayName)
	return obj, nil
}

func (m *Lifecycle) ready(obj *v3.Machine) (*v3.Machine, error) {
	machineDir, err := buildBaseHostDir(obj.Spec.DisplayName)
	if err != nil {
		return obj, err
	}
	logrus.Debugf("Created machine storage directory %s", machineDir)

	config := machineconfig.NewMachineConfig(m.configMapGetter, machineDir, obj.Name)
	defer config.Cleanup()

	if err := config.Restore(); err != nil {
		return obj, err
	}

	// Provision in the background so we can poll and save the config
	done := make(chan error)
	go func() {
		newObj, err := v3.MachineConditionProvisioned.Once(obj, func() (runtime.Object, error) {
			return m.provision(machineDir, obj)
		})
		obj = newObj.(*v3.Machine)
		done <- err
	}()

	// Poll and save config
outer:
	for {
		select {
		case <-done:
			break outer
		case <-time.After(5 * time.Second):
			config.Save()
		}
	}

	newObj, err := v3.MachineConditionConfigSaved.Once(obj, func() (runtime.Object, error) {
		return m.saveConfig(config, machineDir, obj)
	})
	obj = newObj.(*v3.Machine)
	return obj, err
}

func (m *Lifecycle) Updated(obj *v3.Machine) (*v3.Machine, error) {
	if obj.Status.MachineTemplateSpec == nil {
		return obj, nil
	}

	newObj, err := v3.MachineConditionConfigReady.Once(obj, func() (runtime.Object, error) {
		return m.ready(obj)
	})
	obj = newObj.(*v3.Machine)

	return obj, err
}

func (m *Lifecycle) saveConfig(config *machineconfig.MachineConfig, machineDir string, obj *v3.Machine) (*v3.Machine, error) {
	logrus.Infof("Generating and uploading machine config %s", obj.Spec.DisplayName)
	if err := config.Save(); err != nil {
		return obj, err
	}

	ip, err := m.getIP(machineDir, obj)
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
		Address: ip,
		User:    obj.Status.SSHUser,
		Role:    obj.Spec.Roles,
		SSHKey:  sshKey,
	}

	return obj, nil
}

func (m *Lifecycle) getIP(machineDir string, obj *v3.Machine) (string, error) {
	command := buildCommand(machineDir, []string{"ip", obj.Spec.DisplayName})
	output, err := command.Output()
	return string(bytes.TrimSpace(output)), err
}
