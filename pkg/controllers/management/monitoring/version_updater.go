package monitoring

import (
	"context"
	"strings"

	"github.com/rancher/norman/controller"
	pkgns "github.com/rancher/rancher/pkg/namespace"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type VersionUpdator struct {
	settingClient mgmtv3.SettingInterface
	settingLister mgmtv3.SettingLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	updator := &VersionUpdator{
		settingClient: management.Management.Settings(""),
		settingLister: management.Management.Settings("").Controller().Lister(),
	}
	management.Management.CatalogTemplates(pkgns.GlobalNamespace).AddHandler(ctx, "system-app-version-updator", updator.sync)
}

func (u *VersionUpdator) sync(key string, template *mgmtv3.CatalogTemplate) (runtime.Object, error) {
	if template == nil || template.DeletionTimestamp != nil {
		return template, nil
	}

	if template.Spec.CatalogID != "system-library" {
		return template, nil
	}

	templateName := strings.TrimPrefix(template.Name, "system-library-")
	if templateName != "rancher-monitoring" {
		return template, nil
	}

	setting, err := u.settingLister.Get("", "system-monitoring-catalog-id")
	if err != nil {
		return template, &controller.ForgetError{Err: err, Reason: "failed to get setting system-monitoring-catalog-id"}
	}

	var targetVersion *mgmtv3.TemplateVersionSpec
	for i, version := range template.Spec.Versions {
		if version.Version == template.Spec.DefaultVersion {
			targetVersion = &template.Spec.Versions[i]
			break
		}
	}
	if targetVersion.ExternalID == setting.Default {
		return template, nil
	}
	newSetting := setting.DeepCopy()
	newSetting.Default = targetVersion.ExternalID
	_, err = u.settingClient.Update(newSetting)

	return template, err
}
