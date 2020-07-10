package configsyncer

import (
	"sort"
	"strings"

	"github.com/rancher/norman/controller"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	"github.com/rancher/rancher/pkg/project"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func NewConfigGenerator(clusterName string, projectLoggingLister mgmtv3.ProjectLoggingLister, namespaceLister v1.NamespaceLister) *ConfigGenerator {
	return &ConfigGenerator{
		clusterName:          clusterName,
		projectLoggingLister: projectLoggingLister,
		namespaceLister:      namespaceLister,
	}
}

type ConfigGenerator struct {
	clusterName          string
	projectLoggingLister mgmtv3.ProjectLoggingLister
	namespaceLister      v1.NamespaceLister
}

func (s *ConfigGenerator) GenerateClusterLoggingConfig(clusterLogging *mgmtv3.ClusterLogging, systemProjectID, certDir string) ([]byte, error) {
	if clusterLogging == nil {
		return []byte{}, nil
	}

	var excludeNamespaces string
	if clusterLogging.Spec.IncludeSystemComponent != nil && !*clusterLogging.Spec.IncludeSystemComponent {
		var err error
		if excludeNamespaces, err = s.addExcludeNamespaces(systemProjectID); err != nil {
			return nil, err
		}
	}

	buf, err := generator.GenerateClusterConfig(clusterLogging.Spec, excludeNamespaces, certDir)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (s *ConfigGenerator) GenerateProjectLoggingConfig(projectLoggings []*mgmtv3.ProjectLogging, systemProjectID, certDir string) ([]byte, error) {
	if len(projectLoggings) == 0 {
		allProjectLoggings, err := s.projectLoggingLister.List("", labels.NewSelector())
		if err != nil {
			return nil, errors.Wrapf(err, "List project loggings failed")
		}

		for _, logging := range allProjectLoggings {
			if controller.ObjectInCluster(s.clusterName, logging) {
				projectLoggings = append(projectLoggings, logging)
			}
		}

		if len(projectLoggings) == 0 {
			return []byte{}, nil
		}
	}

	sort.Slice(projectLoggings, func(i, j int) bool {
		return projectLoggings[i].Name < projectLoggings[j].Name
	})

	namespaces, err := s.namespaceLister.List(metav1.NamespaceAll, labels.NewSelector())
	if err != nil {
		return nil, errors.Wrap(err, "list namespace failed")
	}

	buf, err := generator.GenerateProjectConfig(projectLoggings, namespaces, systemProjectID, certDir)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (s *ConfigGenerator) addExcludeNamespaces(systemProjectID string) (string, error) {
	namespaces, err := s.namespaceLister.List(metav1.NamespaceAll, labels.NewSelector())
	if err != nil {
		return "", errors.Wrapf(err, "list namespace failed")
	}

	var systemNamespaces []string
	for _, v := range namespaces {
		if v.Annotations[project.ProjectIDAnn] == systemProjectID {
			namespacePattern := loggingconfig.GetNamespacePattern(v.Name)
			systemNamespaces = append(systemNamespaces, namespacePattern)
		}
	}
	sort.Strings(systemNamespaces)
	return strings.Join(systemNamespaces, "|"), nil
}
