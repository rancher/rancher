package generator

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/project"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
)

func generateConfig(templateM, tempalteName string, conf map[string]interface{}) ([]byte, error) {
	tp, err := template.New(tempalteName).Parse(templateM)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err = tp.Execute(buf, conf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GenerateClusterConfig(logging mgmtv3.ClusterLoggingSpec, excludeNamespaces, certDir string) ([]byte, error) {
	wl, err := utils.NewWrapClusterLogging(logging, excludeNamespaces)
	if err != nil {
		return nil, errors.Wrap(err, "to wraper cluster logging failed")
	}

	if wl == nil {
		return []byte{}, nil
	}

	conf := map[string]interface{}{
		"clusterTarget": wl,
		"clusterName":   logging.ClusterName,
		"certDir":       certDir,
	}

	buf, err := generateConfig(ClusterTemplate, loggingconfig.ClusterLevel, conf)
	if err != nil {
		return nil, errors.Wrap(err, "generate cluster config file failed")
	}

	return buf, nil
}

func GenerateProjectConfig(projectLoggings []*mgmtv3.ProjectLogging, namespaces []*k8scorev1.Namespace, systemProjectID, certDir string) ([]byte, error) {
	var wl []utils.ProjectLoggingTemplateWrap
	for _, v := range projectLoggings {
		var grepNamespace []string
		for _, v2 := range namespaces {
			if nsProjectName, ok := v2.Annotations[project.ProjectIDAnn]; ok && nsProjectName == v.Spec.ProjectName {
				grepNamespace = append(grepNamespace, v2.Name)
			}
		}

		if len(grepNamespace) == 0 {
			continue
		}

		formatgrepNamespace := fmt.Sprintf("(%s)", strings.Join(grepNamespace, "|"))
		isSystemProject := v.Spec.ProjectName == systemProjectID
		wpl, err := utils.NewWrapProjectLogging(v.Spec, formatgrepNamespace, isSystemProject)
		if err != nil {
			return nil, err
		}

		if wpl == nil {
			continue
		}

		wl = append(wl, *wpl)
	}

	conf := map[string]interface{}{
		"projectTargets": wl,
		"certDir":        certDir,
	}

	buf, err := generateConfig(ProjectTemplate, loggingconfig.ProjectLevel, conf)
	if err != nil {
		return nil, errors.Wrap(err, "generate project config file failed")
	}

	return buf, nil
}
