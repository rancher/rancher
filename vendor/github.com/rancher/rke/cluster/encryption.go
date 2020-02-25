package cluster

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	ghodssyaml "github.com/ghodss/yaml"
	normantypes "github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/templates"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apiserverconfig "k8s.io/apiserver/pkg/apis/config"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	EncryptionProviderFilePath = "/etc/kubernetes/ssl/encryption.yaml"
)

type encryptionKey struct {
	Name   string
	Secret string
}
type keyList struct {
	KeyList []*encryptionKey
}

func ReconcileEncryptionProviderConfig(ctx context.Context, kubeCluster, currentCluster *Cluster) error {
	if len(kubeCluster.ControlPlaneHosts) == 0 {
		return nil
	}
	// New or existing cluster deployment with encryption enabled. We will rewrite the secrets after deploying the addons.
	if (currentCluster == nil || !currentCluster.IsEncryptionEnabled()) &&
		kubeCluster.IsEncryptionEnabled() {
		kubeCluster.EncryptionConfig.RewriteSecrets = true
		logrus.Debugf("Encryption is enabled in the new spec; have to rewrite secrets")
		return nil
	}
	// encryption is disabled
	if !kubeCluster.IsEncryptionEnabled() && !currentCluster.IsEncryptionEnabled() {
		logrus.Debugf("Encryption is disabled in both current and new spec; no action is required")
		return nil
	}

	// disable encryption
	if !kubeCluster.IsEncryptionEnabled() && currentCluster.IsEncryptionEnabled() {
		logrus.Debugf("Encryption is enabled in the current spec and disabled in the new spec")
		return kubeCluster.DisableSecretsEncryption(ctx, currentCluster, currentCluster.IsEncryptionCustomConfig())
	}

	// encryption configuration updated
	if kubeCluster.IsEncryptionEnabled() && currentCluster.IsEncryptionEnabled() &&
		kubeCluster.EncryptionConfig.EncryptionProviderFile != currentCluster.EncryptionConfig.EncryptionProviderFile {
		kubeCluster.EncryptionConfig.RewriteSecrets = true
		log.Infof(ctx, "[%s] Encryption provider config has changed;"+
			" reconciling cluster's encryption provider configuration", services.ControlRole)
		return services.RestartKubeAPIWithHealthcheck(ctx, kubeCluster.ControlPlaneHosts,
			kubeCluster.LocalConnDialerFactory, kubeCluster.Certificates)
	}

	return nil
}

func (c *Cluster) DisableSecretsEncryption(ctx context.Context, currentCluster *Cluster, custom bool) error {
	log.Infof(ctx, "[%s] Disabling Secrets Encryption..", services.ControlRole)

	if len(c.ControlPlaneHosts) == 0 {
		return nil
	}
	var err error
	if custom {
		c.EncryptionConfig.EncryptionProviderFile, err = currentCluster.generateDisabledCustomEncryptionProviderFile()
	} else {
		c.EncryptionConfig.EncryptionProviderFile, err = currentCluster.generateDisabledEncryptionProviderFile()
	}

	if err != nil {
		return err
	}
	logrus.Debugf("[%s] Deploying Identity first Encryption Provider Configuration", services.ControlRole)
	if err := c.DeployEncryptionProviderFile(ctx); err != nil {
		return err
	}
	if err := services.RestartKubeAPIWithHealthcheck(ctx, c.ControlPlaneHosts, c.LocalConnDialerFactory,
		c.Certificates); err != nil {
		return err
	}
	if err := c.RewriteSecrets(ctx); err != nil {
		return err
	}
	// KubeAPI will be restarted for the last time during controlplane redeployment, since the
	// Configuration file is now empty, the Process Plan will change.
	c.EncryptionConfig.EncryptionProviderFile = ""
	if err := c.DeployEncryptionProviderFile(ctx); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Secrets Encryption is disabled successfully", services.ControlRole)
	return nil
}

func (c *Cluster) RewriteSecrets(ctx context.Context) error {
	log.Infof(ctx, "Rewriting cluster secrets")
	var errgrp errgroup.Group
	k8sClient, err := k8s.NewClient(c.LocalKubeConfigPath, c.K8sWrapTransport)
	if err != nil {
		return fmt.Errorf("failed to initialize new kubernetes client: %v", err)
	}
	secretsList, err := k8s.GetSecretsList(k8sClient, "")
	if err != nil {
		return err
	}

	secretsQueue := util.GetObjectQueue(secretsList.Items)
	for w := 0; w < SyncWorkers; w++ {
		errgrp.Go(func() error {
			var errList []error
			for secret := range secretsQueue {
				s := secret.(v1.Secret)
				err := rewriteSecret(k8sClient, &s)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "Cluster secrets rewritten successfully")
	return nil
}

func (c *Cluster) RotateEncryptionKey(ctx context.Context, fullState *FullState) error {
	//generate new key
	newKey, err := generateEncryptionKey()
	if err != nil {
		return err
	}
	oldKey, err := c.extractActiveKey(c.EncryptionConfig.EncryptionProviderFile)
	if err != nil {
		return err
	}
	// reverse the keys order in the file, making newKey the Active Key
	initialKeyList := []*encryptionKey{ // order is critical here!
		newKey,
		oldKey,
	}
	initialProviderConfig, err := providerFileFromKeyList(keyList{KeyList: initialKeyList})
	if err != nil {
		return err
	}
	c.EncryptionConfig.EncryptionProviderFile = initialProviderConfig
	if err := c.DeployEncryptionProviderFile(ctx); err != nil {
		return err
	}
	// commit to state as soon as possible
	logrus.Debugf("[%s] Updating cluster state", services.ControlRole)
	if err := c.UpdateClusterCurrentState(ctx, fullState); err != nil {
		return err
	}
	if err := services.RestartKubeAPIWithHealthcheck(ctx, c.ControlPlaneHosts, c.LocalConnDialerFactory, c.Certificates); err != nil {
		return err
	}
	// rewrite secrets
	if err := c.RewriteSecrets(ctx); err != nil {
		return err
	}
	// At this point, all secrets have been rewritten using the newKey, so we remove the old one.
	finalKeyList := []*encryptionKey{
		newKey,
	}
	finalProviderConfig, err := providerFileFromKeyList(keyList{KeyList: finalKeyList})
	if err != nil {
		return err
	}
	c.EncryptionConfig.EncryptionProviderFile = finalProviderConfig
	if err := c.DeployEncryptionProviderFile(ctx); err != nil {
		return err
	}
	// commit to state
	logrus.Debugf("[%s] Updating cluster state", services.ControlRole)
	if err := c.UpdateClusterCurrentState(ctx, fullState); err != nil {
		return err
	}
	if err := services.RestartKubeAPIWithHealthcheck(ctx, c.ControlPlaneHosts, c.LocalConnDialerFactory, c.Certificates); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) DeployEncryptionProviderFile(ctx context.Context) error {
	logrus.Debugf("[%s] Deploying Encryption Provider Configuration file on Control Plane nodes..", services.ControlRole)
	return deployFile(ctx, c.ControlPlaneHosts, c.SystemImages.Alpine, c.PrivateRegistriesMap, EncryptionProviderFilePath, c.EncryptionConfig.EncryptionProviderFile)
}

// ReconcileDesiredStateEncryptionConfig We do the rotation outside of the cluster reconcile logic. When we are done,
// DesiredState needs to be updated to reflect the "new" configuration
func (c *Cluster) ReconcileDesiredStateEncryptionConfig(ctx context.Context, fullState *FullState) error {
	fullState.DesiredState.EncryptionConfig = c.EncryptionConfig.EncryptionProviderFile
	return fullState.WriteStateFile(ctx, c.StateFilePath)
}

func (c *Cluster) IsEncryptionEnabled() bool {
	if c == nil {
		return false
	}
	if c.Services.KubeAPI.SecretsEncryptionConfig != nil &&
		c.Services.KubeAPI.SecretsEncryptionConfig.Enabled {
		return true
	}
	return false
}

func (c *Cluster) IsEncryptionCustomConfig() bool {
	if c.IsEncryptionEnabled() &&
		c.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig != nil {
		return true
	}
	return false
}

func (c *Cluster) getEncryptionProviderFile() (string, error) {
	if c.EncryptionConfig.EncryptionProviderFile != "" {
		return c.EncryptionConfig.EncryptionProviderFile, nil
	}
	key, err := generateEncryptionKey()
	if err != nil {
		return "", err
	}
	c.EncryptionConfig.EncryptionProviderFile, err = providerFileFromKeyList(keyList{KeyList: []*encryptionKey{key}})
	return c.EncryptionConfig.EncryptionProviderFile, err
}

func (c *Cluster) extractActiveKey(s string) (*encryptionKey, error) {
	config := apiserverconfig.EncryptionConfiguration{}
	if err := k8s.DecodeYamlResource(&config, c.EncryptionConfig.EncryptionProviderFile); err != nil {
		return nil, err
	}
	resource := config.Resources[0]
	provider := resource.Providers[0]
	return &encryptionKey{
		Name:   provider.AESCBC.Keys[0].Name,
		Secret: provider.AESCBC.Keys[0].Secret,
	}, nil
}

func (c *Cluster) generateDisabledCustomEncryptionProviderFile() (string, error) {
	config := apiserverconfigv1.EncryptionConfiguration{}
	if err := k8s.DecodeYamlResource(&config, c.EncryptionConfig.EncryptionProviderFile); err != nil {
		return "", err
	}

	// 1. Prepend custom config providers with ignore provider
	updatedProviders := []apiserverconfigv1.ProviderConfiguration{{
		Identity: &apiserverconfigv1.IdentityConfiguration{},
	}}

	for _, provider := range config.Resources[0].Providers {
		if provider.Identity != nil {
			continue
		}
		updatedProviders = append(updatedProviders, provider)
	}

	config.Resources[0].Providers = updatedProviders

	// 2. Generate custom config file
	jsonConfig, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	yamlConfig, err := sigsyaml.JSONToYAML(jsonConfig)
	if err != nil {
		return "", nil
	}

	return string(yamlConfig), nil
}

func (c *Cluster) generateDisabledEncryptionProviderFile() (string, error) {
	key, err := c.extractActiveKey(c.EncryptionConfig.EncryptionProviderFile)
	if err != nil {
		return "", err
	}
	return disabledProviderFileFromKey(key)
}

func rewriteSecret(k8sClient *kubernetes.Clientset, secret *v1.Secret) error {
	var err error
	if err = k8s.UpdateSecret(k8sClient, secret); err == nil {
		return nil
	}
	if apierrors.IsConflict(err) {
		secret, err = k8s.GetSecret(k8sClient, secret.Name, secret.Namespace)
		if err != nil {
			return err
		}
		err = k8s.UpdateSecret(k8sClient, secret)
	}
	return err
}

func generateEncryptionKey() (*encryptionKey, error) {
	// TODO: do this in a better way
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return &encryptionKey{
		Name:   normantypes.GenerateName("key"),
		Secret: base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%X", buf))),
	}, nil
}

func isEncryptionEnabled(rkeConfig *v3.RancherKubernetesEngineConfig) bool {
	if rkeConfig.Services.KubeAPI.SecretsEncryptionConfig != nil &&
		rkeConfig.Services.KubeAPI.SecretsEncryptionConfig.Enabled {
		return true
	}
	return false
}
func isEncryptionCustomConfig(rkeConfig *v3.RancherKubernetesEngineConfig) bool {
	if isEncryptionEnabled(rkeConfig) &&
		rkeConfig.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig != nil {
		return true
	}
	return false
}

func providerFileFromKeyList(keyList interface{}) (string, error) {
	return templates.CompileTemplateFromMap(templates.MultiKeyEncryptionProviderFile, keyList)
}

func disabledProviderFileFromKey(keyList interface{}) (string, error) {
	return templates.CompileTemplateFromMap(templates.DisabledEncryptionProviderFile, keyList)
}

func (c *Cluster) readEncryptionCustomConfig() (string, error) {
	// directly marshalling apiserverconfig.EncryptionConfiguration to yaml breaks things because TypeMeta
	// is nested and all fields don't have tags. apiserverconfigv1 has json tags only. So we do this as a work around.

	out := apiserverconfigv1.EncryptionConfiguration{}
	err := apiserverconfigv1.Convert_config_EncryptionConfiguration_To_v1_EncryptionConfiguration(
		c.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig, &out, nil)
	if err != nil {
		return "", err
	}
	jsonConfig, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	yamlConfig, err := sigsyaml.JSONToYAML(jsonConfig)
	if err != nil {
		return "", nil
	}

	return templates.CompileTemplateFromMap(templates.CustomEncryptionProviderFile,
		struct{ CustomConfig string }{CustomConfig: string(yamlConfig)})
}

func resolveCustomEncryptionConfig(clusterFile string) (string, *apiserverconfig.EncryptionConfiguration, error) {
	var err error
	var r map[string]interface{}
	err = ghodssyaml.Unmarshal([]byte(clusterFile), &r)
	if err != nil {
		return clusterFile, nil, fmt.Errorf("error unmarshalling: %v", err)
	}
	services, ok := r["services"].(map[string]interface{})
	if services == nil || !ok {
		return clusterFile, nil, nil
	}
	kubeapi, ok := services["kube-api"].(map[string]interface{})
	if kubeapi == nil || !ok {
		return clusterFile, nil, nil
	}
	sec, ok := kubeapi["secrets_encryption_config"].(map[string]interface{})
	if sec == nil || !ok {
		return clusterFile, nil, nil
	}
	customConfig, ok := sec["custom_config"].(map[string]interface{})

	if ok && customConfig != nil {
		delete(sec, "custom_config")
		newClusterFile, err := ghodssyaml.Marshal(r)
		c, err := parseCustomConfig(customConfig)
		return string(newClusterFile), c, err
	}
	return clusterFile, nil, nil
}

func parseCustomConfig(customConfig map[string]interface{}) (*apiserverconfig.EncryptionConfiguration, error) {
	var err error

	data, err := json.Marshal(customConfig)
	if err != nil {
		return nil, fmt.Errorf("error marshalling: %v", err)
	}
	scheme := runtime.NewScheme()
	err = apiserverconfig.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("error adding to scheme: %v", err)
	}
	err = apiserverconfigv1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("error adding to scheme: %v", err)
	}
	codecs := serializer.NewCodecFactory(scheme)
	decoder := codecs.UniversalDecoder()
	decodedObj, objType, err := decoder.Decode(data, nil, nil)

	if err != nil {
		return nil, fmt.Errorf("error decoding data: %v", err)
	}

	decodedConfig, ok := decodedObj.(*apiserverconfig.EncryptionConfiguration)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", objType)
	}
	return decodedConfig, nil
}
