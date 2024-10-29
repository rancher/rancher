package rancher

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mcuadros/go-version"
	"github.com/rancher/norman/condition"
	"github.com/rancher/rancher/pkg/api/norman/customization/cred"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/data"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/summary"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	cattleNamespace                                 = "cattle-system"
	forceUpgradeLogoutConfig                        = "forceupgradelogout"
	forceLocalSystemAndDefaultProjectCreation       = "forcelocalprojectcreation"
	forceSystemNamespacesAssignment                 = "forcesystemnamespaceassignment"
	migrateFromMachineToPlanSecret                  = "migratefrommachinetoplanesecret"
	migrateEncryptionKeyRotationLeaderToStatus      = "migrateencryptionkeyrotationleadertostatus"
	migrateDynamicSchemaToMachinePools              = "migratedynamicschematomachinepools"
	migrateRKEClusterState                          = "migraterkeclusterstate"
	migrateSystemAgentVarDirToDataDirectory         = "migratesystemagentvardirtodatadirectory"
	migrateHarvesterCloudCredentialExpirationConfig = "migrateharvestercloudcredentialexpiration"
	rancherVersionKey                               = "rancherVersion"
	projectsCreatedKey                              = "projectsCreated"
	namespacesAssignedKey                           = "namespacesAssigned"
	capiMigratedKey                                 = "capiMigrated"
	encryptionKeyRotationStatusMigratedKey          = "encryptionKeyRotationStatusMigrated"
	dynamicSchemaMachinePoolsMigratedKey            = "dynamicSchemaMachinePoolsMigrated"
	rkeClustersAnnotatedForMigrationKey             = "rkeClustersAnnotatedForMigration"
	systemAgentVarDirMigratedKey                    = "systemAgentVarDirMigrated"
	harvesterCloudCredentialExpirationMigratedKey   = "harvesterCloudCredentialExpirationMigrated"
)

func runMigrations(wranglerContext *wrangler.Context) error {
	if err := forceUpgradeLogout(wranglerContext.Core.ConfigMap(), wranglerContext.Mgmt.Token(), "v2.6.0"); err != nil {
		return err
	}

	if err := forceSystemAndDefaultProjectCreation(wranglerContext.Core.ConfigMap(), wranglerContext.Mgmt.Cluster()); err != nil {
		return err
	}

	if features.MCM.Enabled() {
		if err := forceSystemNamespaceAssignment(wranglerContext.Core.ConfigMap(), wranglerContext.Mgmt.Project()); err != nil {
			return err
		}
	}

	if features.RKE2.Enabled() {
		// must migrate system agent data directory first, since update requests will be rejected by webhook if
		// "CATTLE_AGENT_VAR_DIR" is set within AgentEnvVars.
		if err := migrateSystemAgentDataDirectory(wranglerContext); err != nil {
			return err
		}
		if err := migrateCAPIMachineLabelsAndAnnotationsToPlanSecret(wranglerContext); err != nil {
			return err
		}
		if err := migrateEncryptionKeyRotationLeader(wranglerContext); err != nil {
			return err
		}
		if err := migrateMachinePoolsDynamicSchemaLabel(wranglerContext); err != nil {
			return err
		}
	}

	if features.Harvester.Enabled() {
		if err := migrateHarvesterCloudCredentialExpiration(wranglerContext); err != nil {
			return err
		}
	}

	return migrateRKEClusterStates(wranglerContext)
}

func getConfigMap(configMapController controllerv1.ConfigMapController, configMapName string) (*v1.ConfigMap, error) {
	cm, err := configMapController.Cache().Get(cattleNamespace, configMapName)
	if err != nil && !k8serror.IsNotFound(err) {
		return nil, err
	}

	// if this is the first ever migration initialize the configmap
	if cm == nil {
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: cattleNamespace,
			},
			Data: make(map[string]string, 1),
		}
	}

	// we do not migrate in development environments
	if rancherversion.Version == "dev" {
		return nil, nil
	}

	return cm, nil
}

func createOrUpdateConfigMap(configMapClient controllerv1.ConfigMapClient, cm *v1.ConfigMap) error {
	var err error
	if cm.ObjectMeta.GetResourceVersion() != "" {
		_, err = configMapClient.Update(cm)
	} else {
		_, err = configMapClient.Create(cm)
	}

	return err
}

// forceUpgradeLogout will delete all dashboard tokens forcing a logout.  This is useful when there is a major frontend
// upgrade and we want all users to be sent to a central point.  This function will check for the `forceUpgradeLogoutConfig`
// configuration map and only run if the last migrated version is lower than the given `migrationVersion`.
func forceUpgradeLogout(configMapController controllerv1.ConfigMapController, tokenController v3.TokenController, migrationVersion string) error {
	cm, err := getConfigMap(configMapController, forceUpgradeLogoutConfig)
	if err != nil || cm == nil {
		return err
	}

	// if no last migration is found we always run force logout
	if lastMigration, ok := cm.Data[rancherVersionKey]; ok {

		// if a valid sem ver is found we only migrate if the version is less than the target version
		if semver.IsValid(lastMigration) && semver.IsValid(rancherversion.Version) && version.Compare(migrationVersion, lastMigration, "<=") {
			return nil
		}

		// if an unknown format is given we migrate any time the current version does not equal the last migration
		if lastMigration == rancherversion.Version {
			return nil
		}
	}

	logrus.Infof("Detected %s upgrade, forcing logout for all web users", migrationVersion)

	// list all tokens that were created for the dashboard
	allTokens, err := tokenController.Cache().List(labels.SelectorFromSet(labels.Set{tokens.TokenKindLabel: "session"}))
	if err != nil {
		logrus.Error("Failed to list tokens for upgrade forced logout")
		return err
	}

	// log out all the dashboard users forcing them to be redirected to the login page
	for _, token := range allTokens {
		err = tokenController.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil && !k8serror.IsNotFound(err) {
			logrus.Errorf("Failed to delete token [%s] for upgrade forced logout", token.Name)
		}
	}

	cm.Data[rancherVersionKey] = rancherversion.Version
	return createOrUpdateConfigMap(configMapController, cm)
}

// forceSystemAndDefaultProjectCreation will set the corresponding conditions on the local cluster object,
// if it exists, to Unknown. This will force the corresponding controller to check that the projects exist
// and create them, if necessary.
func forceSystemAndDefaultProjectCreation(configMapController controllerv1.ConfigMapController, clusterClient v3.ClusterClient) error {
	cm, err := getConfigMap(configMapController, forceLocalSystemAndDefaultProjectCreation)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[projectsCreatedKey] == "true" {
		return nil
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		localCluster, err := clusterClient.Get("local", metav1.GetOptions{})
		if err != nil {
			if k8serror.IsNotFound(err) {
				return nil
			}
			return err
		}

		v32.ClusterConditionDefaultProjectCreated.Unknown(localCluster)
		v32.ClusterConditionSystemProjectCreated.Unknown(localCluster)

		_, err = clusterClient.Update(localCluster)
		return err
	}); err != nil {
		return err
	}

	cm.Data[projectsCreatedKey] = "true"
	return createOrUpdateConfigMap(configMapController, cm)
}

func forceSystemNamespaceAssignment(configMapController controllerv1.ConfigMapController, projectClient v3.ProjectClient) error {
	cm, err := getConfigMap(configMapController, forceSystemNamespacesAssignment)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[namespacesAssignedKey] == rancherversion.Version {
		return nil
	}

	err = applyProjectConditionForNamespaceAssignment("authz.management.cattle.io/system-project=true", v32.ProjectConditionSystemNamespacesAssigned, projectClient)
	if err != nil {
		return err
	}
	err = applyProjectConditionForNamespaceAssignment("authz.management.cattle.io/default-project=true", v32.ProjectConditionDefaultNamespacesAssigned, projectClient)
	if err != nil {
		return err
	}

	cm.Data[namespacesAssignedKey] = rancherversion.Version
	return createOrUpdateConfigMap(configMapController, cm)
}

func applyProjectConditionForNamespaceAssignment(label string, condition condition.Cond, projectClient v3.ProjectClient) error {
	projects, err := projectClient.List("", metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return err
	}

	for i := range projects.Items {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			p := &projects.Items[i]
			p, err = projectClient.Get(p.Namespace, p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			condition.Unknown(p)
			_, err = projectClient.Update(p)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// migrateRKEClusterStates sets the `RKEForceUpdate` annotation on all management cluster objects for
// RKE-provisioned clusters.
func migrateRKEClusterStates(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateRKEClusterState)
	if err != nil {
		return fmt.Errorf("error getting configmap %s: %w", migrateRKEClusterState, err)
	} else if cm == nil {
		return nil
	}

	// Check if this migration has already run.
	if cm.Data[rkeClustersAnnotatedForMigrationKey] == "true" {
		return nil
	}

	// Collect all RKE clusters that need migration.
	mgmtClusters, err := w.Mgmt.Cluster().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing management clusters: %w", err)
	}

	// Mark all RKE clusters for migration.
	for _, cluster := range mgmtClusters.Items {
		// Skip this cluster if it's not RKE-provisioned.
		if cluster.Spec.RancherKubernetesEngineConfig == nil {
			continue
		}

		// Retry the cluster object status update on conflict in case something else is updating it at the same time.
		if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			c, err := w.Mgmt.Cluster().Get(cluster.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting cluster %s: %w", cluster.Name, err)
			}

			clusterCopy := c.DeepCopy()
			clusterCopy.Annotations[clusterprovisioner.RKEForceUpdate] = "true"

			if _, err = w.Mgmt.Cluster().Update(clusterCopy); err != nil {
				return fmt.Errorf("error updating cluster %s: %w", cluster.Name, err)
			}

			return nil
		}); err != nil {
			logrus.Errorf("error updating annotation for cluster %s: %s", cluster.Name, err)
			return err
		}
	}

	cm.Data = map[string]string{
		rkeClustersAnnotatedForMigrationKey: "true",
	}

	// Update the configmap that indicates that this migration is complete.
	if err := retry.OnError(retry.DefaultBackoff, func(err error) bool {
		// Retry all errors.
		return true
	}, func() error {
		return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
	}); err != nil {
		return fmt.Errorf("error updating configmap %s: %w", cm.Name, err)
	}

	return nil
}

func migrateCAPIMachineLabelsAndAnnotationsToPlanSecret(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateFromMachineToPlanSecret)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[capiMigratedKey] == "true" {
		return nil
	}

	mgmtClusters, err := w.Mgmt.Cluster().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	bootstrapLabelExcludes := map[string]struct{}{
		capr.InitNodeMachineIDLabel: {},
		capr.InitNodeLabel:          {},
	}

	boostrapAnnotationExcludes := map[string]struct{}{
		capr.DrainAnnotation:     {},
		capr.DrainDoneAnnotation: {},
		capr.JoinURLAnnotation:   {},
		capr.PostDrainAnnotation: {},
		capr.PreDrainAnnotation:  {},
		capr.UnCordonAnnotation:  {},
	}

	for _, mgmtCluster := range mgmtClusters.Items {
		provClusters, err := w.Provisioning.Cluster().List(mgmtCluster.Spec.FleetWorkspaceName, metav1.ListOptions{})
		if k8serror.IsNotFound(err) || len(provClusters.Items) == 0 {
			continue
		} else if err != nil {
			return err
		}

		for _, provCluster := range provClusters.Items {
			machines, err := w.CAPI.Machine().List(provCluster.Namespace, metav1.ListOptions{LabelSelector: labels.Set{capi.ClusterNameLabel: provCluster.Name}.String()})
			if err != nil {
				return err
			}

			otherMachines, err := w.CAPI.Machine().List(provCluster.Namespace, metav1.ListOptions{LabelSelector: labels.Set{capr.ClusterNameLabel: provCluster.Name}.String()})
			if err != nil {
				return err
			}

			allMachines := append(machines.Items, otherMachines.Items...)

			for _, machine := range allMachines {
				if machine.Spec.Bootstrap.ConfigRef == nil || machine.Spec.Bootstrap.ConfigRef.APIVersion != capr.RKEAPIVersion {
					continue
				}

				planSecrets, err := w.Core.Secret().List(machine.Namespace, metav1.ListOptions{LabelSelector: labels.Set{capr.MachineNameLabel: machine.Name}.String()})
				if err != nil {
					return err
				}
				if len(planSecrets.Items) == 0 {
					continue
				}

				for _, secret := range planSecrets.Items {
					if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						secret, err := w.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						secret = secret.DeepCopy()
						capr.CopyMap(secret.Labels, machine.Labels)
						capr.CopyMap(secret.Annotations, machine.Annotations)
						_, err = w.Core.Secret().Update(secret)
						return err
					}); err != nil {
						return err
					}
				}

				if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
					bootstrap, err := w.RKE.RKEBootstrap().Get(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					bootstrap = bootstrap.DeepCopy()
					capr.CopyMapWithExcludes(bootstrap.Labels, machine.Labels, bootstrapLabelExcludes)
					capr.CopyMapWithExcludes(bootstrap.Annotations, machine.Annotations, boostrapAnnotationExcludes)
					if bootstrap.Spec.ClusterName == "" {
						// If the bootstrap spec cluster name is blank, we need to update the bootstrap spec to the correct value
						// This is to handle old rkebootstrap objects for unmanaged clusters that did not have the spec properly set
						if v, ok := bootstrap.Labels[capi.ClusterNameLabel]; ok && v != "" {
							bootstrap.Spec.ClusterName = v
						}
					}
					_, err = w.RKE.RKEBootstrap().Update(bootstrap)
					return err
				}); err != nil {
					return err
				}

				if machine.Spec.InfrastructureRef.APIVersion == capr.RKEAPIVersion || machine.Spec.InfrastructureRef.APIVersion == capr.RKEMachineAPIVersion {
					gv, err := schema.ParseGroupVersion(machine.Spec.InfrastructureRef.APIVersion)
					if err != nil {
						// This error should not occur because RKEAPIVersion and RKEMachineAPIVersion are valid
						continue
					}

					gvk := schema.GroupVersionKind{
						Group:   gv.Group,
						Version: gv.Version,
						Kind:    machine.Spec.InfrastructureRef.Kind,
					}
					if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						infraMachine, err := w.Dynamic.Get(gvk, machine.Spec.InfrastructureRef.Namespace, machine.Spec.InfrastructureRef.Name)
						if err != nil {
							return err
						}

						d, err := data.Convert(infraMachine.DeepCopyObject())
						if err != nil {
							return err
						}

						if changed, err := insertOrUpdateCondition(d, summary.NewCondition("Ready", "True", "", "")); err != nil {
							return err
						} else if changed {
							_, err = w.Dynamic.UpdateStatus(&unstructured.Unstructured{Object: d})
							return err
						}
						return err
					}); err != nil {
						return err
					}
				}
			}
		}
	}

	cm.Data[capiMigratedKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

func migrateEncryptionKeyRotationLeader(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateEncryptionKeyRotationLeaderToStatus)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[encryptionKeyRotationStatusMigratedKey] == "true" {
		return nil
	}

	mgmtClusters, err := w.Mgmt.Cluster().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mgmtCluster := range mgmtClusters.Items {
		if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			cp, err := w.RKE.RKEControlPlane().Get(mgmtCluster.Spec.FleetWorkspaceName, mgmtCluster.Name, metav1.GetOptions{})
			if k8serror.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			leader := cp.Annotations["rke.cattle.io/encrypt-key-rotation-leader"]
			if leader == "" {
				return nil
			}
			cp = cp.DeepCopy()
			cp.Status.RotateEncryptionKeysLeader = leader
			cp, err = w.RKE.RKEControlPlane().UpdateStatus(cp)
			if err != nil {
				return err
			}
			cp = cp.DeepCopy()
			delete(cp.Annotations, "rke.cattle.io/encrypt-key-rotation-leader")
			cp, err = w.RKE.RKEControlPlane().Update(cp)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}

	cm.Data[encryptionKeyRotationStatusMigratedKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

func migrateMachinePoolsDynamicSchemaLabel(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateDynamicSchemaToMachinePools)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[dynamicSchemaMachinePoolsMigratedKey] == "true" {
		return nil
	}

	mgmtClusters, err := w.Mgmt.Cluster().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mgmtCluster := range mgmtClusters.Items {
		provClusters, err := w.Provisioning.Cluster().List(mgmtCluster.Spec.FleetWorkspaceName, metav1.ListOptions{})
		if k8serror.IsNotFound(err) || len(provClusters.Items) == 0 {
			continue
		} else if err != nil {
			return err
		}
		for _, provCluster := range provClusters.Items {
			if provCluster.Spec.RKEConfig == nil {
				continue
			}
			if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				cluster, err := w.Provisioning.Cluster().Get(provCluster.Namespace, provCluster.Name, metav1.GetOptions{})
				if k8serror.IsNotFound(err) {
					return nil
				} else if err != nil {
					return err
				}
				cluster = cluster.DeepCopy()
				// search for machine pools without the `dynamic-schema-spec` annotation and apply it
				for i, machinePool := range cluster.Spec.RKEConfig.MachinePools {
					var spec v32.DynamicSchemaSpec
					if machinePool.DynamicSchemaSpec != "" && json.Unmarshal([]byte(machinePool.DynamicSchemaSpec), &spec) == nil {
						continue
					}
					nodeConfig := machinePool.NodeConfig
					if nodeConfig == nil {
						return fmt.Errorf("machine pool node config must not be nil")
					}
					apiVersion := nodeConfig.APIVersion
					if apiVersion != capr.DefaultMachineConfigAPIVersion && apiVersion != "" {
						continue
					}
					ds, err := w.Mgmt.DynamicSchema().Get(strings.ToLower(nodeConfig.Kind), metav1.GetOptions{})
					if err != nil {
						return err
					}
					specJSON, err := json.Marshal(ds.Spec)
					if err != nil {
						return err
					}
					cluster.Spec.RKEConfig.MachinePools[i].DynamicSchemaSpec = string(specJSON)
				}
				_, err = w.Provisioning.Cluster().Update(cluster)
				if err != nil {
					return err
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	cm.Data[dynamicSchemaMachinePoolsMigratedKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

func migrateSystemAgentDataDirectory(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateSystemAgentVarDirToDataDirectory)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[systemAgentVarDirMigratedKey] == "true" {
		return nil
	}

	provClusters, err := w.Provisioning.Cluster().List("", metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, cluster := range provClusters.Items {
		systemAgentDataDir := ""
		envVars := make([]rkev1.EnvVar, 0, len(cluster.Spec.AgentEnvVars))
		for _, e := range cluster.Spec.AgentEnvVars {
			if e.Name == capr.SystemAgentDataDirEnvVar {
				// don't break, the webhook allows duplicate entries and the last one would have been the effective data dir
				systemAgentDataDir = e.Value
			} else {
				envVars = append(envVars, e)
			}
		}
		if systemAgentDataDir == "" {
			continue
		}

		cluster = *cluster.DeepCopy()
		cluster.Spec.AgentEnvVars = envVars
		cluster.Spec.RKEConfig.DataDirectories.SystemAgent = systemAgentDataDir
		_, err = w.Provisioning.Cluster().Update(&cluster)
		if err != nil {
			return err
		}
	}

	cm.Data[systemAgentVarDirMigratedKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

// migrateHarvesterCloudCredentialExpiration will add an expiration timestamp to all harvester cloud credential secrets
// that are based on the v3 Token API. For each credential, the kubeconfig is extracted, the token is derived from the
// kubeconfig, and the cred.CloudCredentialExpirationAnnotation is inserted with a value of the token's `ExpiresAt`
// field converted to milliseconds since Unix Epoch. If the credential is not a harvester credential, this
// function does nothing. This migration is only performed once, after which the expiration annotation is managed by
// norman for all v3 API CRUD.
func migrateHarvesterCloudCredentialExpiration(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateHarvesterCloudCredentialExpirationConfig)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[harvesterCloudCredentialExpirationMigratedKey] == "true" {
		return nil
	}

	secrets, err := w.Core.Secret().List("cattle-global-data", metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, secret := range secrets.Items {
		if kubeconfigYaml, ok := secret.Data["harvestercredentialConfig-kubeconfigContent"]; ok && kubeconfigYaml != nil {

			expiration, err := cred.GetHarvesterCloudCredentialExpirationFromKubeconfig(string(kubeconfigYaml), func(tokenName string) (*v32.Token, error) {
				return w.Mgmt.Token().Get(tokenName, metav1.GetOptions{})
			})
			if apierrors.IsNotFound(err) {
				logrus.Debugf("Cloud credential [%s] using nonexistent token", secret.Name)
				// if the secret is not found, we use the unix 0 timestamp. This is done a bit odd since
				// time.Unix returns an int64 and we don't want to produce overflow
				expiration = strconv.FormatInt(time.Unix(0, 0).UnixMilli(), 10)
			} else if err != nil {
				return fmt.Errorf("failed to get harvester cloud credential expiration from kubeconfig: %w", err)
			}

			if expiration != "" {
				secret = *secret.DeepCopy()
				secret.Annotations[cred.CloudCredentialExpirationAnnotation] = expiration
				_, err = w.Core.Secret().Update(&secret)
				if err != nil {
					return err
				}
			}
		}
	}

	cm.Data[harvesterCloudCredentialExpirationMigratedKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

func insertOrUpdateCondition(d data.Object, desiredCondition summary.Condition) (bool, error) {
	for _, cond := range summary.GetUnstructuredConditions(d) {
		if desiredCondition.Equals(cond) {
			return false, nil
		}
	}

	// The conditions must be converted to a map so that DeepCopyJSONValue will
	// recognize it as a map instead of a data.Object.
	newCond, err := convert.EncodeToMap(desiredCondition.Object)
	if err != nil {
		return false, err
	}

	dConditions := d.Slice("status", "conditions")
	conditions := make([]interface{}, len(dConditions))
	found := false
	for i, cond := range dConditions {
		if cond.String("type") == desiredCondition.Type() {
			conditions[i] = newCond
			found = true
		} else {
			conditions[i], err = convert.EncodeToMap(cond)
			if err != nil {
				return false, err
			}
		}
	}

	if !found {
		conditions = append(conditions, newCond)
	}
	d.SetNested(conditions, "status", "conditions")

	return true, nil
}
