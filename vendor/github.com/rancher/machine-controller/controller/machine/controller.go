package machine

import (
	"bytes"
	"os"
	"strings"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(management *config.ManagementContext) {
	machineLifecycle := &Lifecycle{}
	machineClient := management.Management.Machines("")
	machineLifecycle.machineClient = machineClient
	machineLifecycle.machineTemplateClient = management.Management.MachineTemplates("")

	machineClient.
		Controller().
		AddHandler(v3.NewMachineLifecycleAdapter("machine-controller", machineClient, machineLifecycle))
}

type Lifecycle struct {
	machineClient         v3.MachineInterface
	machineTemplateClient v3.MachineTemplateInterface
}

func (m *Lifecycle) Create(obj *v3.Machine) (*v3.Machine, error) {
	// No need to create a deepcopy of obj, obj is already a deepcopy
	return m.createOrUpdate(obj)
}

func (m *Lifecycle) Updated(obj *v3.Machine) (*v3.Machine, error) {
	// YOU MUST CALL DEEPCOPY
	objCopy := obj.DeepCopy()
	return m.createOrUpdate(objCopy)
}

func (m *Lifecycle) Remove(obj *v3.Machine) (*v3.Machine, error) {
	// No need to create a deepcopy of obj, obj is already a deepcopy
	if obj.Spec.Driver == "" {
		return nil, nil
	}
	machineDir, err := buildBaseHostDir(obj.Name)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Creating machine storage directory %s", machineDir)
	err = restoreMachineDir(obj, machineDir)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(machineDir)

	mExists, err := machineExists(machineDir, obj.Name)
	if err != nil {
		return nil, err
	}

	if mExists {
		logrus.Infof("Removing machine %s", obj.Name)
		if err := deleteMachine(machineDir, obj); err != nil {
			return nil, err
		}
		logrus.Infof("Removing machine %s done", obj.Name)
	}
	return obj, nil
}

func (m *Lifecycle) createOrUpdate(obj *v3.Machine) (*v3.Machine, error) {
	if obj.Spec.Driver == "" {
		return nil, nil
	}
	if obj.Status.Provisioned && obj.Status.ExtractedConfig != "" {
		return nil, nil
	}
	machineDir, err := buildBaseHostDir(obj.Name)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Creating machine storage directory %s", machineDir)
	if !obj.Status.Provisioned {
		configRawMap := map[string]interface{}{}
		if obj.Spec.MachineTemplateName != "" {
			machineTemplate, err := m.machineTemplateClient.Get(obj.Spec.MachineTemplateName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			for k, v := range machineTemplate.Spec.PublicValues {
				configRawMap[k] = v
			}
			for k, v := range machineTemplate.Spec.SecretValues {
				configRawMap[k] = v
			}
		} else {
			var err error
			switch obj.Spec.Driver {
			case "amazonec2":
				configRawMap, err = toMap(obj.Spec.AmazonEC2Config)
				if err != nil {
					return nil, err
				}
			case "digitalocean":
				configRawMap, err = toMap(obj.Spec.DigitalOceanConfig)
				if err != nil {
					return nil, err
				}
			case "azure":
				configRawMap, err = toMap(obj.Spec.AzureConfig)
				if err != nil {
					return nil, err
				}
			}
		}

		createCommandsArgs := buildCreateCommand(obj, configRawMap)
		cmd := buildCommand(machineDir, createCommandsArgs)
		logrus.Infof("Provisioning machine %s", obj.Name)
		// at the beginning of provisioning we set status to unknown
		if err := m.updateMachineCondition(obj, v1.ConditionUnknown, ProvisionedState, ""); err != nil {
			return nil, err
		}
		stdoutReader, stderrReader, err := startReturnOutput(cmd)
		if err != nil {
			return nil, err
		}
		defer cmd.Wait()
		hostExist := false
		if err := m.reportStatus(stdoutReader, stderrReader, obj); err != nil {
			if strings.Contains(err.Error(), "Host already exists") {
				hostExist = true
			}
			if !hostExist {
				if err := m.updateMachineCondition(obj, v1.ConditionFalse, ProvisionedState, err.Error()); err != nil {
					return nil, err
				}
				return nil, err
			}
		}
		if err := cmd.Wait(); err != nil {
			if !hostExist {
				if err := m.updateMachineCondition(obj, v1.ConditionFalse, ProvisionedState, err.Error()); err != nil {
					return nil, err
				}
				return nil, err
			}
		}
		obj, err = m.machineClient.Get(obj.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		obj.Status.Provisioned = true
		if obj, err = m.machineClient.Update(obj); err != nil {
			return nil, err
		}
		logrus.Infof("Provisioning machine %s done", obj.Name)
	}
	if obj.Status.ExtractedConfig == "" {
		logrus.Infof("Generating and uploading machine config %s", obj.Name)
		if err := waitUntilSSHKey(machineDir, obj); err != nil {
			return nil, err
		}
		sshkey, err := getSSHPrivateKey(machineDir, obj)
		if err != nil {
			return nil, err
		}
		destFile, err := createExtractedConfig(machineDir, obj)
		if err != nil {
			return nil, err
		}
		extractedConf, err := encodeFile(destFile)
		if err != nil {
			return nil, err
		}
		command := buildCommand(machineDir, []string{"ip", obj.Name})
		output, err := command.Output()
		if err != nil {
			return nil, err
		}
		ip := string(bytes.TrimSpace(output))
		obj, err = m.machineClient.Get(obj.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		obj.Status.Address = ip
		obj.Status.ExtractedConfig = extractedConf
		obj.Status.SSHPrivateKey = sshkey
		sshUser := ""
		switch obj.Spec.Driver {
		case "amazonec2":
			sshUser = obj.Spec.AmazonEC2Config.SSHUser
		case "digitalocean":
			sshUser = obj.Spec.DigitalOceanConfig.SSHUser
		case "azure":
			sshUser = obj.Spec.AzureConfig.SSHUser
		}
		obj.Status.SSHUser = sshUser
		if obj, err = m.machineClient.Update(obj); err != nil {
			return nil, err
		}
		if err := m.updateMachineCondition(obj, v1.ConditionTrue, ProvisionedState, "Machine is ready"); err != nil {
			return nil, err
		}
		logrus.Infof("Generating and uploading machine config %s done", obj.Name)
	}
	os.RemoveAll(machineDir)
	return obj, nil
}

func (m *Lifecycle) updateMachineCondition(obj *v3.Machine, status v1.ConditionStatus, state, reason string) error {
	var err error
	obj, err = m.machineClient.Get(obj.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	obj.Status.Conditions = append(obj.Status.Conditions, v3.MachineCondition{
		LastTransitionTime: now,
		LastUpdateTime:     now,
		Type:               state,
		Status:             status,
		Reason:             reason,
	})
	if _, err := m.machineClient.Update(obj); err != nil {
		return err
	}
	return nil
}
