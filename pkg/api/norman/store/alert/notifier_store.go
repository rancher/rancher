package notifier

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	smtpSecretKey     = "smtpCredentialSecret"
	wechatSecretKey   = "wechatCredentialSecret"
	dingtalkSecretKey = "dingtalkCredentialSecret"
)

type Store struct {
	types.Store
	clusterLister  v3.ClusterLister
	secretMigrator *secretmigrator.Migrator
}

func NewNotifier(mgmt *config.ScaledContext, store types.Store) *Store {
	return &Store{
		Store:         store,
		clusterLister: mgmt.Management.Clusters("").Controller().Lister(),
		secretMigrator: secretmigrator.NewMigrator(
			mgmt.Core.Secrets("").Controller().Lister(),
			mgmt.Core.Secrets(""),
		),
	}
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	smtpSecret, wechatSecret, dingtalkSecret, err := s.migrateSecrets(data, "", "", "")
	if err != nil {
		return nil, err
	}
	data, err = s.Store.Create(apiContext, schema, data)
	if err != nil {
		if smtpSecret != nil {
			if cleanupErr := s.secretMigrator.Cleanup(smtpSecret.Name); cleanupErr != nil {
				logrus.Errorf("notifier store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
		if wechatSecret != nil {
			if cleanupErr := s.secretMigrator.Cleanup(wechatSecret.Name); cleanupErr != nil {
				logrus.Errorf("notifier store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
		if dingtalkSecret != nil {
			if cleanupErr := s.secretMigrator.Cleanup(dingtalkSecret.Name); cleanupErr != nil {
				logrus.Errorf("notifier store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
		return nil, err
	}
	clusterID := data["clusterId"].(string)
	var owner metav1.OwnerReference
	if smtpSecret != nil || wechatSecret != nil || dingtalkSecret != nil {
		cluster, err := s.clusterLister.Get("", clusterID)
		if err != nil {
			return nil, err
		}
		owner = metav1.OwnerReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
			Name:       clusterID,
			UID:        cluster.UID,
		}
	}
	if smtpSecret != nil {
		err = s.secretMigrator.UpdateSecretOwnerReference(smtpSecret, owner)
		if err != nil {
			logrus.Errorf("notifier store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
		}
	}
	if wechatSecret != nil {
		err = s.secretMigrator.UpdateSecretOwnerReference(wechatSecret, owner)
		if err != nil {
			logrus.Errorf("notifier store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
		}
	}
	if dingtalkSecret != nil {
		err = s.secretMigrator.UpdateSecretOwnerReference(dingtalkSecret, owner)
		if err != nil {
			logrus.Errorf("notifier store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
		}
	}
	return data, nil
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	existing, err := s.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	currentSMTPSecret, _ := existing[smtpSecretKey].(string)
	currentWechatSecret, _ := existing[wechatSecretKey].(string)
	currentDingtalkSecret, _ := existing[dingtalkSecretKey].(string)
	smtpSecret, wechatSecret, dingtalkSecret, err := s.migrateSecrets(data, currentSMTPSecret, currentWechatSecret, currentDingtalkSecret)
	if err != nil {
		return nil, err
	}

	data, err = s.Store.Update(apiContext, schema, data, id)
	if err != nil {
		if smtpSecret != nil && currentSMTPSecret == "" {
			if cleanupErr := s.secretMigrator.Cleanup(smtpSecret.Name); cleanupErr != nil {
				logrus.Errorf("notifier store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
		if wechatSecret != nil && currentWechatSecret == "" {
			if cleanupErr := s.secretMigrator.Cleanup(wechatSecret.Name); cleanupErr != nil {
				logrus.Errorf("notifier store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
		if dingtalkSecret != nil {
			if cleanupErr := s.secretMigrator.Cleanup(dingtalkSecret.Name); cleanupErr != nil {
				logrus.Errorf("notifier store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
	}
	return data, err
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	existing, err := s.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	secrets := []string{smtpSecretKey, wechatSecretKey, dingtalkSecretKey}
	for _, sec := range secrets {
		if secretName, ok := existing[sec]; ok {
			err = s.secretMigrator.Cleanup(secretName.(string))
			if err != nil {
				return nil, err
			}
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}

func (s *Store) migrateSecrets(data map[string]interface{}, currentSMTP, currentWechat, currentDingtalk string) (*corev1.Secret, *corev1.Secret, *corev1.Secret, error) {
	smtpConfig, err := getSMTPConfig(data)
	if err != nil {
		return nil, nil, nil, err
	}
	smtpSecret, err := s.secretMigrator.CreateOrUpdateSMTPSecret(currentSMTP, smtpConfig, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if smtpSecret != nil {
		data[smtpSecretKey] = smtpSecret.Name
		smtpConfig.Password = ""
		data["smtpConfig"], err = convert.EncodeToMap(smtpConfig)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	wechatConfig, err := getWechatConfig(data)
	if err != nil {
		return nil, nil, nil, err
	}
	wechatSecret, err := s.secretMigrator.CreateOrUpdateWechatSecret(currentWechat, wechatConfig, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if wechatSecret != nil {
		data[wechatSecretKey] = wechatSecret.Name
		wechatConfig.Secret = ""
		data["wechatConfig"], err = convert.EncodeToMap(wechatConfig)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	dingtalkConfig, err := getDingtalkConfig(data)
	if err != nil {
		return nil, nil, nil, err
	}
	dingtalkSecret, err := s.secretMigrator.CreateOrUpdateDingtalkSecret(currentDingtalk, dingtalkConfig, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if dingtalkSecret != nil {
		data[dingtalkSecretKey] = dingtalkSecret.Name
		dingtalkConfig.Secret = ""
		data["dingtalkConfig"], err = convert.EncodeToMap(dingtalkConfig)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return smtpSecret, wechatSecret, dingtalkSecret, nil
}

func getConfig(data map[string]interface{}, key string, ret interface{}) (interface{}, error) {
	config := values.GetValueN(data, key)
	if config == nil {
		return nil, nil
	}
	err := convert.ToObj(config, &ret)
	return ret, err
}

func getSMTPConfig(data map[string]interface{}) (*apimgmtv3.SMTPConfig, error) {
	smtpConfig, err := getConfig(data, "smtpConfig", &apimgmtv3.SMTPConfig{})
	if err != nil || smtpConfig == nil {
		return nil, err
	}
	return smtpConfig.(*apimgmtv3.SMTPConfig), nil
}

func getWechatConfig(data map[string]interface{}) (*apimgmtv3.WechatConfig, error) {
	wechatConfig, err := getConfig(data, "wechatConfig", &apimgmtv3.WechatConfig{})
	if err != nil || wechatConfig == nil {
		return nil, err
	}
	return wechatConfig.(*apimgmtv3.WechatConfig), nil
}

func getDingtalkConfig(data map[string]interface{}) (*apimgmtv3.DingtalkConfig, error) {
	dingtalkConfig, err := getConfig(data, "dingtalkConfig", &apimgmtv3.DingtalkConfig{})
	if err != nil || dingtalkConfig == nil {
		return nil, err
	}
	return dingtalkConfig.(*apimgmtv3.DingtalkConfig), nil
}
