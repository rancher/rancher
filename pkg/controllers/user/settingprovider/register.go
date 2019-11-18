package settingprovider

import (
	"context"
	"fmt"
	"os"

	csetting "github.com/rancher/rancher/pkg/controllers/user/setting"
	"github.com/rancher/rancher/pkg/settings"
	v1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(ctx context.Context, workload *config.UserOnlyContext) error {
	sp := &settingsProvider{
		secrets:       workload.Core.Secrets(""),
		secretsLister: workload.Core.Secrets("").Controller().Lister(),
	}

	if err := settings.SetProvider(sp); err != nil {
		return err
	}

	return nil
}

type settingsProvider struct {
	secrets       v1.SecretInterface
	secretsLister v1.SecretLister
	fallback      map[string]string
}

func (s *settingsProvider) Get(name string) string {
	value := os.Getenv(settings.GetEnvKey(name))
	if value != "" {
		return value
	}
	obj, err := s.secretsLister.Get(csetting.CattleNamespace, csetting.DefaultSettingSecretName)
	if err != nil {
		return s.fallback[name]
	}

	if obj.Data[name] == nil {
		return s.fallback[name]
	}

	return string(obj.Data[name])
}

func (s *settingsProvider) Set(name, value string) error {
	envValue := os.Getenv(settings.GetEnvKey(name))
	if envValue != "" {
		return fmt.Errorf("setting %s can not be set because it is from environment variable", name)
	}
	obj, err := s.secrets.GetNamespaced(csetting.CattleNamespace, csetting.DefaultSettingSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	stringData := map[string]string{name: value}
	obj.StringData = stringData
	_, err = s.secrets.Update(obj)
	return err
}

func (s *settingsProvider) SetIfUnset(name, value string) error {
	obj, err := s.secrets.GetNamespaced(csetting.CattleNamespace, csetting.DefaultSettingSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if obj.Data[name] != nil {
		return nil
	}

	obj.StringData[name] = value
	_, err = s.secrets.Update(obj)
	return err
}

func (s *settingsProvider) SetAll(settingsMap map[string]settings.Setting) error {
	fallback := map[string]string{}
	obj, err := s.secrets.GetNamespaced(csetting.CattleNamespace, csetting.DefaultSettingSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			obj = csetting.SetDefaultSettingSecret(csetting.DefaultSettingSecretName, csetting.CattleNamespace)
			_, err := s.secrets.Create(obj)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	stringData := make(map[string]string)
	for name, setting := range settingsMap {
		key := settings.GetEnvKey(name)
		value := os.Getenv(key)

		if obj.Data[name] == nil {
			stringData[name] = setting.Default
			if value != "" {
				stringData[name] = value
			}
		} else {
			if value != "" && string(obj.Data[name]) != value {
				stringData[name] = value
			}
		}
		fallback[name] = stringData[name]
	}
	obj.StringData = stringData
	_, err = s.secrets.Update(obj)
	if err != nil {
		return err
	}

	s.fallback = fallback
	return nil
}
