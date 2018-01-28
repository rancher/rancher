package settings

import (
	"os"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(context *config.ManagementContext) error {
	sp := &settingsProvider{
		settings:       context.Management.Settings(""),
		settingsLister: context.Management.Settings("").Controller().Lister(),
	}

	return settings.SetProvider(sp)
}

type settingsProvider struct {
	settings       v3.SettingInterface
	settingsLister v3.SettingLister
	fallback       map[string]string
}

func (s *settingsProvider) Get(name string) string {
	obj, err := s.settingsLister.Get("", name)
	if err != nil {
		return s.fallback[name]
	}
	if obj.Value == "" {
		return obj.Default
	}
	return obj.Value
}

func (s *settingsProvider) Set(name, value string) error {
	obj, err := s.settings.Get(name, v1.GetOptions{})
	if err != nil {
		return err
	}

	obj.Value = value
	_, err = s.settings.Update(obj)
	return err
}

func (s *settingsProvider) SetAll(settings map[string]settings.Setting) error {
	fallback := map[string]string{}

	for name, setting := range settings {
		key := "CATTLE_" + strings.ToUpper(strings.Replace(name, "-", "_", -1))
		value := os.Getenv(key)

		obj, err := s.settings.Get(setting.Name, v1.GetOptions{})
		if errors.IsNotFound(err) {
			newSetting := &v3.Setting{
				ObjectMeta: v1.ObjectMeta{
					Name: setting.Name,
				},
				Default: setting.Default,
			}
			if value != "" {
				newSetting.Value = value
			}
			fallback[newSetting.Name] = newSetting.Value
			_, err := s.settings.Create(newSetting)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			update := false
			if obj.Default != setting.Default {
				obj.Default = setting.Default
				update = true
			}
			if value != "" && obj.Value != value {
				obj.Value = value
				update = true
			}
			fallback[obj.Name] = obj.Value
			if update {
				_, err := s.settings.Update(obj)
				if err != nil {
					return err
				}
			}
		}
	}

	s.fallback = fallback

	return nil
}
