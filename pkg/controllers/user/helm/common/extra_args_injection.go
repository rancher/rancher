package common

import (
	"net/url"
	"strings"

	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type InjectAppArgsFunc func(obj *v3.App) (content map[string]string)

const (
	systemCatalogName = "system-library"
)

var (
	extraArgsFuncs = []InjectAppArgsFunc{
		injectDefaultRegistry,
		injectClusterInfo,
	}
)

func injectDefaultRegistry(obj *v3.App) map[string]string {
	values, err := url.Parse(obj.Spec.ExternalID)
	if err != nil {
		logrus.Errorf("check catalog type failed: %s", err.Error())
	}

	catalogWithNamespace := values.Query().Get("catalog")
	split := strings.SplitN(catalogWithNamespace, "/", 2)
	catalog := split[len(split)-1]

	reg := settings.SystemDefaultRegistry.Get()
	if catalog != systemCatalogName || reg == "" {
		return nil
	}
	return map[string]string{"systemDefaultRegistry": reg}
}

func injectClusterInfo(obj *v3.App) map[string]string {
	clusterName, projectName := ref.Parse(obj.Spec.ProjectName)
	return map[string]string{
		"clusterName": clusterName,
		"projectName": projectName,
	}
}

func GetExtraArgs(app *v3.App) map[string]string {
	rtn := map[string]string{}
	for _, afunc := range extraArgsFuncs {
		content := afunc(app)
		for k, v := range content {
			rtn["global."+k] = v
		}
	}
	return rtn
}
