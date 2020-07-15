package generator

import (
	"bytes"
	"sort"
	"strings"
	"text/template"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"

	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
)

var tmplCache = template.New("template")

func init() {
	tmplCache = tmplCache.Funcs(template.FuncMap{"escapeString": escapeString})
	tmplCache = template.Must(tmplCache.Parse(SourceTemplate))
	tmplCache = template.Must(tmplCache.Parse(FilterTemplate))
	tmplCache = template.Must(tmplCache.Parse(MatchTemplate))
	tmplCache = template.Must(tmplCache.Parse(Template))
}

func GenerateConfig(tempalteName string, conf interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := tmplCache.ExecuteTemplate(buf, tempalteName, conf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GenerateClusterConfig(logging v32.ClusterLoggingSpec, excludeNamespaces, certDir string) ([]byte, error) {
	wl, err := newWrapClusterLogging(logging, excludeNamespaces, certDir)
	if err != nil {
		return nil, errors.Wrap(err, "to wraper cluster logging failed")
	}

	if wl == nil {
		return []byte{}, nil
	}

	if logging.SyslogConfig != nil && logging.SyslogConfig.Token != "" {
		if err = ValidateSyslogToken(wl); err != nil {
			return nil, err
		}
	}

	if len(logging.OutputTags) != 0 {
		if err = ValidateCustomTags(wl); err != nil {
			return nil, err
		}
	}

	validateData := *wl
	if logging.FluentForwarderConfig != nil && wl.EnableShareKey {
		validateData.EnableShareKey = false //skip generate precan configure included ruby code
	}
	if err = ValidateCustomTarget(validateData); err != nil {
		return nil, err
	}

	buf, err := GenerateConfig("cluster-template", wl)
	if err != nil {
		return nil, errors.Wrap(err, "generate cluster config file failed")
	}

	return buf, nil
}

func GenerateProjectConfig(projectLoggings []*mgmtv3.ProjectLogging, namespaces []*k8scorev1.Namespace, systemProjectID, certDir string) ([]byte, error) {
	var wl []ProjectLoggingTemplateWrap
	for _, v := range projectLoggings {
		var containerSourcePath []string
		for _, v2 := range namespaces {
			if nsProjectName, ok := v2.Annotations[project.ProjectIDAnn]; ok && nsProjectName == v.Spec.ProjectName {
				sourcePathPattern := loggingconfig.GetNamespacePathPattern(v2.Name)
				containerSourcePath = append(containerSourcePath, sourcePathPattern)
			}
		}

		if len(containerSourcePath) == 0 {
			continue
		}

		sort.Strings(containerSourcePath)
		containerSourcePaths := strings.Join(containerSourcePath, ",")
		isSystemProject := v.Spec.ProjectName == systemProjectID
		wpl, err := newWrapProjectLogging(v.Spec, containerSourcePaths, certDir, isSystemProject)
		if err != nil {
			return nil, err
		}

		if wpl == nil {
			continue
		}

		if wpl.SyslogConfig.Token != "" {
			if err = ValidateSyslogToken(wpl); err != nil {
				return nil, err
			}
		}

		if len(wpl.OutputTags) != 0 {
			if err = ValidateCustomTags(wpl); err != nil {
				return nil, err
			}
		}

		validateData := *wpl
		if v.Spec.FluentForwarderConfig != nil && wpl.EnableShareKey {
			validateData.EnableShareKey = false //skip generate precan configure included ruby code
		}
		if err = ValidateCustomTarget(validateData); err != nil {
			return nil, err
		}

		wl = append(wl, *wpl)
	}

	buf, err := GenerateConfig("project-template", wl)
	if err != nil {
		return nil, errors.Wrap(err, "generate project config file failed")
	}

	return buf, nil
}
