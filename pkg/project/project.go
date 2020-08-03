package project

import (
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	System  = "System"
	Default = "Default"
)

const (
	SystemImageVersionAnn = "field.cattle.io/systemImageVersion"
	ProjectIDAnn          = "field.cattle.io/projectId"
)

var (
	SystemProjectLabel = map[string]string{"authz.management.cattle.io/system-project": "true"}
)

func GetSystemProject(clusterName string, projectLister mgmtv3.ProjectLister) (*mgmtv3.Project, error) {
	projects, err := projectLister.List(clusterName, labels.Set(SystemProjectLabel).AsSelector())
	if err != nil {
		return nil, errors.Wrapf(err, "list project failed")
	}

	if len(projects) == 0 {
		return nil, errors.New("can't find system project")
	}

	return projects[0], nil
}
