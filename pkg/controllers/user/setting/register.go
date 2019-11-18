package setting

import (
	"context"
	"fmt"

	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// SettingProviderController listens for setting CUD in management API
// and propagates the changes to the secret of cattle-system namespace in cluster API
type Controller struct {
	secrets                 v1.SecretInterface
	secretsLister           v1.SecretLister
	managementSettingLister v3.SettingLister
	clusterName             string
}

const (
	CattleNamespace          = "cattle-system"
	DefaultSettingSecretName = "cattle-setting-provider"
	settingSecretAnnotation  = "secret.setting.cattle.io/secret"
	LocalClusterName         = "local"
)

var syncSettings = []string{"ingress-ip-domain", "rdns-base-url"}

func Register(ctx context.Context, cluster *config.UserContext) {
	c := &Controller{
		secrets:                 cluster.Core.Secrets(""),
		secretsLister:           cluster.Core.Secrets("").Controller().Lister(),
		managementSettingLister: cluster.Management.Management.Settings("").Controller().Lister(),
		clusterName:             cluster.ClusterName,
	}

	if cluster.ClusterName == LocalClusterName {
		return
	}

	sync := v3.NewSettingLifecycleAdapter(fmt.Sprintf("settingProviderController_%s", cluster.ClusterName), true,
		cluster.Management.Management.Settings(""), c)

	cluster.Management.Management.Settings("").AddHandler(ctx, "settingProviderController",
		func(key string, obj *v3.Setting) (runtime.Object, error) {
			if obj == nil {
				return sync(key, nil)
			}

			// only propagate settings within the sync settings array
			_, found := find(syncSettings, obj.Name)
			if !found {
				return nil, nil
			}

			if obj.Labels != nil {
				if obj.Labels["cattle.io/creator"] == "norman" {
					return sync(key, obj)
				}
			}

			return nil, nil
		})
}

func (c *Controller) Create(obj *v3.Setting) (runtime.Object, error) {
	return nil, c.createOrUpdate(obj, CattleNamespace)
}

func (c *Controller) Updated(obj *v3.Setting) (runtime.Object, error) {
	return nil, c.createOrUpdate(obj, CattleNamespace)
}

func (c *Controller) Remove(obj *v3.Setting) (runtime.Object, error) {
	settingSecret, err := c.secretExistsForSetting(DefaultSettingSecretName, CattleNamespace)
	if err != nil {
		return nil, err
	}
	if settingSecret == nil {
		logrus.Infof("Default setting secret [%s] not exist in namespace [%s], skip to delete",
			DefaultSettingSecretName, CattleNamespace)
		return nil, nil
	}
	toUpdate := settingSecret.DeepCopy()
	logrus.Infof("Deleting setting data [%s] in secret [%s]", obj.Name, DefaultSettingSecretName)
	delete(toUpdate.Data, obj.Name)
	_, err = c.secrets.Update(toUpdate)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *Controller) createOrUpdate(obj *v3.Setting, ns string) error {
	secret, err := c.secretExistsForSetting(DefaultSettingSecretName, ns)
	if err != nil {
		return err
	}

	logrus.Debugf("Copying setting [%s:%s] into secret [%s] at namespace [%s]",
		obj.Name, obj.Value, DefaultSettingSecretName, ns)

	// create a new secret and store setting value if not exist
	if secret == nil {
		secret = SetDefaultSettingSecret(DefaultSettingSecretName, ns)
		toCreate := updateSettingSecretStringData(secret, obj)
		_, err := c.secrets.Create(toCreate)
		if err != nil {
			return err
		}
		return nil
	}

	// update existing secret with new setting value
	toUpdate := secret.DeepCopy()
	toUpdate = updateSettingSecretStringData(toUpdate, obj)
	_, err = c.secrets.Update(toUpdate)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) secretExistsForSetting(name, ns string) (*corev1.Secret, error) {
	obj, err := c.secretsLister.Get(ns, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if obj.DeletionTimestamp != nil {
		return nil, nil
	}

	return obj, nil
}

func SetDefaultSettingSecret(name, namespace string) *corev1.Secret {
	defaultSettingSecret := &corev1.Secret{}
	defaultSettingSecret.Name = name
	defaultSettingSecret.Namespace = namespace
	defaultSettingSecret.Type = corev1.SecretTypeOpaque
	defaultSettingSecret.Annotations = make(map[string]string)
	defaultSettingSecret.StringData = make(map[string]string)
	defaultSettingSecret.Annotations[settingSecretAnnotation] = "true"
	return defaultSettingSecret
}

func updateSettingSecretStringData(secret *corev1.Secret, setting *v3.Setting) *corev1.Secret {
	var updateValue = setting.Value
	if updateValue == "" {
		updateValue = setting.Default
	}
	stringData := map[string]string{setting.Name: updateValue}
	secret.StringData = stringData
	return secret
}

func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}
