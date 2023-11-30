// Package settings registers the settings provider which acts as a client for setting and provides functions for
// maintaining settings in k8s on startup. This maintenance includes configuring setting to match any corresponding
// env variables that are set or updating their default values.
package settings

import (
	"fmt"
	"os"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(settingController managementcontrollers.SettingController) error {
	sp := &settingsProvider{
		settings:     settingController,
		settingCache: settingController.Cache(),
	}

	return settings.SetProvider(sp)
}

type settingsProvider struct {
	settings     managementcontrollers.SettingClient
	settingCache managementcontrollers.SettingCache
	fallback     map[string]string
}

func (s *settingsProvider) Get(name string) string {
	value := os.Getenv(settings.GetEnvKey(name))
	if value != "" {
		return value
	}
	obj, err := s.settingCache.Get(name)
	if err != nil {
		val, err := s.settings.Get(name, metav1.GetOptions{})
		if err != nil {
			return s.fallback[name]
		}
		obj = val
	}
	if obj.Value == "" {
		return obj.Default
	}
	return obj.Value
}

func (s *settingsProvider) Set(name, value string) error {
	envValue := os.Getenv(settings.GetEnvKey(name))
	if envValue != "" {
		return fmt.Errorf("setting %s can not be set because it is from environment variable", name)
	}
	obj, err := s.settings.Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	obj.Value = value
	_, err = s.settings.Update(obj)
	return err
}

func (s *settingsProvider) SetIfUnset(name, value string) error {
	obj, err := s.settings.Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if obj.Value != "" {
		return nil
	}

	obj.Value = value
	_, err = s.settings.Update(obj)
	return err
}

// SetAll iterates through a map of settings.Setting and updates corresponding settings in k8s
// to match any values set for them via their respective CATTLE_<setting-name> env var, their
// source to "env" if configured by an env var, and their default to match the setting in the
// map.
func (s *settingsProvider) SetAll(settingsMap map[string]settings.Setting) error {
	fallback := map[string]string{}

	for name, setting := range settingsMap {
		key := settings.GetEnvKey(name)
		envValue, envOk := os.LookupEnv(key)

		obj, err := s.settings.Get(setting.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			newSetting := &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{
					Name: setting.Name,
				},
				Default: setting.Default,
			}
			if envOk {
				newSetting.Source = "env"
				newSetting.Value = envValue
			}
			if newSetting.Value == "" {
				fallback[newSetting.Name] = newSetting.Default
			} else {
				fallback[newSetting.Name] = newSetting.Value
			}
			_, err := s.settings.Create(newSetting)
			// Rancher will race in an HA setup to try and create the settings
			// so if it exists just move on.
			if err != nil && !errors.IsAlreadyExists(err) {
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
			if envOk && obj.Source != "env" {
				obj.Source = "env"
				update = true
			}
			if !envOk && obj.Source == "env" {
				obj.Source = ""
				update = true
			}
			if envOk && obj.Value != envValue {
				obj.Value = envValue
				update = true
			}
			if obj.Value == "" {
				fallback[obj.Name] = obj.Default
			} else {
				fallback[obj.Name] = obj.Value
			}
			if update {
				_, err := s.settings.Update(obj)
				if err != nil {
					return err
				}
			}
		}
	}

	s.fallback = fallback

	if err := s.cleanupUnknownSettings(settingsMap); err != nil {
		logrus.Errorf("Error cleaning up unknown settings: %v", err)
	}

	return nil
}

const unknownSettingLabelKey = "cattle.io/unknown"

// cleanupUnknownSettings lists all the settings in the cluster and cleans up all unknown (e.g. deprecated) settings.
// At the moment, we just mark such settings with a label.
// In the future release we'll be removing them.
func (s *settingsProvider) cleanupUnknownSettings(settingsMap map[string]settings.Setting) error {
	list, err := s.settings.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range list.Items {
		if _, ok := settingsMap[item.Name]; ok {
			continue
		}

		if err := s.markSettingAsUnknown(item.Name); err != nil {
			logrus.Errorf("Error adding label for setting %s: %v", item.Name, err)
			continue
		}
	}

	return nil
}

// markSettingAsUnknown adds a label to the setting to mark it as unknown.
func (s *settingsProvider) markSettingAsUnknown(name string) error {
	logrus.Warnf("Unknown setting %s", name)

	st, err := s.settings.Get(name, metav1.GetOptions{}) // Deliberately re-fetch the setting.
	if err != nil {
		return err
	}

	if st.Labels == nil {
		st.Labels = map[string]string{}
	}
	st.Labels[unknownSettingLabelKey] = "true"

	_, err = s.settings.Update(st)

	return err
}
