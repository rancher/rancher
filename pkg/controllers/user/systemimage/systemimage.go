package systemimage

import (
	"encoding/json"
	"fmt"

	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

var systemProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/system-project": "true"})

type Syncer struct {
	clusterName      string
	daemonsets       v1beta2.DaemonSetInterface
	daemonsetLister  v1beta2.DaemonSetLister
	deployments      v1beta2.DeploymentInterface
	deploymentLister v1beta2.DeploymentLister
	projectLister    v3.ProjectLister
	projects         v3.ProjectInterface
}

func (s *Syncer) Sync(key string, obj *v3.Project) error {
	projects, err := s.projectLister.List(s.clusterName, systemProjectLabels.AsSelector())
	if err != nil {
		return fmt.Errorf("get projects failed when try to upgrade system tools, %v", err)
	}

	var systemProject *v3.Project
	for _, v := range projects {
		if v.Spec.DisplayName == project.System {
			systemProject = v.DeepCopy()
		}
	}

	if systemProject == nil {
		return nil
	}

	versionMap := make(map[string]string)
	curSysImageVersion := systemProject.Annotations[project.SystemImageVersionAnn]
	if curSysImageVersion != "" {
		if err = json.Unmarshal([]byte(curSysImageVersion), &versionMap); err != nil {
			return fmt.Errorf("unmashal current system service version failed, %v", err)
		}
	}

	for k, v := range systemServices {
		newVersion, err := v.Upgrade(versionMap[k])
		if err != nil {
			return err
		}
		versionMap[k] = newVersion
	}

	newVersion, err := json.Marshal(versionMap)
	if err != nil {
		return fmt.Errorf("marshal new system service version %v failed, %v", versionMap, err)
	}

	systemProject.Annotations[project.SystemImageVersionAnn] = string(newVersion)
	_, err = s.projects.Update(systemProject)
	return err
}

func GetSystemImageVersion() (string, error) {
	versionMap := make(map[string]string)
	for k, v := range systemServices {
		version, err := v.Version()
		if err != nil {
			return "", err
		}
		versionMap[k] = version
	}

	b, err := json.Marshal(versionMap)
	if err != nil {
		return "", fmt.Errorf("marshal toolsSystemImages failed: %v", err)
	}

	return string(b), nil
}
