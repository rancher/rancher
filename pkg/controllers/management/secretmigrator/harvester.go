package secretmigrator

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rke "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AuthorizedSecretAnnotation                        = "v2prov-secret-authorized-for-cluster"
	AuthorizedSecretDeletesOnClusterRemovalAnnotation = "v2prov-authorized-secret-deletes-on-cluster-removal"
)

func (h *handler) syncHarvesterCloudConfig(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Spec.RKEConfig == nil || cluster.Name == "local" {
		return cluster, nil
	}

	if v3.ClusterConditionHarvesterCloudProviderConfigMigrated.IsTrue(cluster) || !h.migrator.isHarvesterCluster(cluster) {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	updatedRKEConfig := cluster.Spec.RKEConfig.DeepCopy()
	initialRKEConfig := cluster.Spec.RKEConfig.DeepCopy()

	harvesterConfigSecrets, err := h.migrator.migrateHarvesterCloudProviderConfig(updatedRKEConfig, cluster, cluster.Name)
	if err != nil {
		return cluster, err
	}

	if len(harvesterConfigSecrets) != 0 {
		cluster.Spec.RKEConfig = updatedRKEConfig
		cluster, err = h.provisioningClusters.Update(cluster)
		if err != nil {
			h.migrator.CleanupKnownSecrets(harvesterConfigSecrets)
			cluster.Spec.RKEConfig = initialRKEConfig
			return cluster, err
		}
	}

	v3.ClusterConditionHarvesterCloudProviderConfigMigrated.True(cluster)
	cluster, err = h.provisioningClusters.UpdateStatus(cluster)
	if err != nil {
		h.migrator.CleanupKnownSecrets(harvesterConfigSecrets)
		cluster.Spec.RKEConfig = initialRKEConfig
		return cluster, err
	}
	return cluster, nil
}

func (m *Migrator) migrateHarvesterCloudProviderConfig(rkeConfig *v1.RKEConfig, owner runtime.Object, clusterName string) ([]*corev1.Secret, error) {
	if rkeConfig == nil || len(rkeConfig.MachineSelectorConfig) == 0 {
		return nil, nil
	}

	secrets := make([]*corev1.Secret, 0, len(rkeConfig.MachineSelectorConfig))
	for i := range rkeConfig.MachineSelectorConfig {
		secret, err := m.createOrUpdateHarvesterCloudProviderConfigSecret(clusterName, rkeConfig.MachineSelectorConfig[i], owner)
		if err != nil {
			m.CleanupKnownSecrets(secrets)
			return nil, err
		}

		if secret != nil {
			secrets = append(secrets, secret)
			rkeConfig.MachineSelectorConfig[i].Config.Data["cloud-provider-config"] = fmt.Sprintf("secret://%s:%s", secret.Namespace, secret.Name)
		}
	}

	return secrets, nil
}

func (m *Migrator) createOrUpdateHarvesterCloudProviderConfigSecret(clusterName string, machineSelectorConfig rke.RKESystemConfig, owner runtime.Object) (*corev1.Secret, error) {
	name, nameFound := machineSelectorConfig.Config.Data["cloud-provider-name"]
	if !nameFound {
		return nil, nil
	}

	if strings.ToLower(convert.ToString(name)) != "harvester" {
		return nil, nil
	}

	cloudConfig, cloudProviderConfigFound := machineSelectorConfig.Config.Data["cloud-provider-config"]
	if !cloudProviderConfigFound {
		return nil, nil
	}

	// don't create a new secret if one has already been provided.
	if strings.HasPrefix(convert.ToString(cloudConfig), "secret://") {
		return nil, nil
	}

	annotation := map[string]string{
		AuthorizedSecretAnnotation: clusterName,
	}

	return m.CreateOrUpdateHarvesterCloudConfigSecret("", convert.ToString(cloudConfig), annotation, owner, "cloud-provider-config")
}
