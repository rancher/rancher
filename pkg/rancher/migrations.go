package rancher

import (
	"bytes"

	"github.com/mcuadros/go-version"
	"github.com/rancher/norman/condition"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	controllerapiextv1 "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	v1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	cattleNamespace                           = "cattle-system"
	forceUpgradeLogoutConfig                  = "forceupgradelogout"
	forceLocalSystemAndDefaultProjectCreation = "forcelocalprojectcreation"
	forceSystemNamespacesAssignment           = "forcesystemnamespaceassignment"
	migrateFromMachineToPlanSecret            = "migratefrommachinetoplanesecret"
	rancherVersionKey                         = "rancherVersion"
	projectsCreatedKey                        = "projectsCreated"
	namespacesAssignedKey                     = "namespacesAssigned"
	caSecretName                              = "tls-ca-additional"
	caSecretField                             = "ca-additional.pem"
	capiMigratedKey                           = "capiMigrated"
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

	if features.EmbeddedClusterAPI.Enabled() {
		if err := addWebhookConfigToCAPICRDs(wranglerContext.CRD.CustomResourceDefinition()); err != nil {
			return err
		}
	}

	if features.RKE2.Enabled() {
		if err := migrateCAPIMachineLabelsAndAnnotationsToPlanSecret(
			wranglerContext.Core.ConfigMap(),
			wranglerContext.Core.Secret(),
			wranglerContext.Mgmt.Cluster().Cache(),
			wranglerContext.Provisioning.Cluster().Cache(),
			wranglerContext.CAPI.Machine().Cache(),
			wranglerContext.RKE.RKEBootstrap(),
		); err != nil {
			return err
		}
	}

	return copyCAAdditionalSecret(wranglerContext.Core.Secret())
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

		v32.ClusterConditionconditionDefaultProjectCreated.Unknown(localCluster)
		v32.ClusterConditionconditionSystemProjectCreated.Unknown(localCluster)

		_, err = clusterClient.Update(localCluster)
		return err
	}); err != nil {
		return err
	}

	cm.Data[projectsCreatedKey] = "true"
	return createOrUpdateConfigMap(configMapController, cm)
}

// copyCAAdditionalSecret will ensure that if a secret named tls-ca-additional exists in the cattle-system namespace
// then this secret also exists in the fleet-default namespace. This is because the machine creation and deletion jobs
// need to have access to this secret as well.
func copyCAAdditionalSecret(secretClient controllerv1.SecretClient) error {
	cattleSecret, err := secretClient.Get("cattle-system", caSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			return nil
		}
		return err
	}

	fleetSecret, err := secretClient.Get(fleetconst.ClustersDefaultNamespace, caSecretName, metav1.GetOptions{})
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return err
		}
		fleetSecret.Name = cattleSecret.Name
		fleetSecret.Namespace = fleetconst.ClustersDefaultNamespace
	} else if bytes.Equal(fleetSecret.Data[caSecretField], cattleSecret.Data[caSecretField]) {
		// Both secrets contain the same data.
		return nil
	}

	fleetSecret.Data = cattleSecret.Data

	if err != nil {
		// In this case, the fleetSecret doesn't exist yet.
		_, err = secretClient.Create(fleetSecret)
	} else {
		_, err = secretClient.Update(fleetSecret)
	}

	return err

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

func addWebhookConfigToCAPICRDs(crdClient controllerapiextv1.CustomResourceDefinitionClient) error {
	crds, err := crdClient.List(metav1.ListOptions{
		LabelSelector: "auth.cattle.io/cluster-indexed=true",
	})
	if err != nil {
		return err
	}
	for _, crd := range crds.Items {
		if crd.Spec.Group != "cluster.x-k8s.io" || (crd.Spec.Conversion != nil && crd.Spec.Conversion.Strategy != apiextv1.NoneConverter) {
			continue
		}
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestCrd, err := crdClient.Get(crd.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			latestCrd.Spec.Conversion = &apiextv1.CustomResourceConversion{
				Strategy: apiextv1.WebhookConverter,
				Webhook: &apiextv1.WebhookConversion{
					ClientConfig: &apiextv1.WebhookClientConfig{
						Service: &apiextv1.ServiceReference{
							Namespace: "cattle-system",
							Name:      "webhook-service",
							Path:      &[]string{"/convert"}[0],
							Port:      &[]int32{443}[0],
						},
					},
					ConversionReviewVersions: []string{"v1", "v1beta1"},
				},
			}
			_, err = crdClient.Update(&crd)
			return err
		}); err != nil {
			return err
		}
	}

	return nil
}

func migrateCAPIMachineLabelsAndAnnotationsToPlanSecret(configMapController controllerv1.ConfigMapController, secretsController controllerv1.SecretController, mgmtClusterCache v3.ClusterCache,
	provClusterCache provv1.ClusterCache, capiMachineCache capicontrollers.MachineCache, bootstrapClient rkecontrollers.RKEBootstrapClient) error {
	cm, err := getConfigMap(configMapController, migrateFromMachineToPlanSecret)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[capiMigratedKey] == "true" {
		return nil
	}

	mgmtClusters, err := mgmtClusterCache.List(labels.Everything())
	if err != nil {
		return err
	}

	bootstrapLabelExcludes := map[string]struct{}{
		rke2.InitNodeMachineIDLabel:     {},
		rke2.InitNodeMachineIDDoneLabel: {},
		rke2.InitNodeLabel:              {},
	}

	boostrapAnnotationExcludes := map[string]struct{}{
		rke2.DrainAnnotation:     {},
		rke2.DrainDoneAnnotation: {},
		rke2.JoinURLAnnotation:   {},
		rke2.PostDrainAnnotation: {},
		rke2.PreDrainAnnotation:  {},
		rke2.UnCordonAnnotation:  {},
	}

	for _, mgmtCluster := range mgmtClusters {
		provClusters, err := provClusterCache.GetByIndex(cluster.ByCluster, mgmtCluster.Name)
		if k8serror.IsNotFound(err) || len(provClusters) == 0 {
			continue
		} else if err != nil {
			return err
		}

		for _, provCluster := range provClusters {
			machines, err := capiMachineCache.List(provCluster.Namespace, labels.Set{capi.ClusterLabelName: provCluster.Name}.AsSelector())
			if err != nil {
				return err
			}

			for _, machine := range machines {
				if machine.Spec.Bootstrap.ConfigRef == nil || machine.Spec.Bootstrap.ConfigRef.APIVersion != rke2.RKEAPIVersion {
					continue
				}

				planSecrets, err := secretsController.Cache().List(machine.Namespace, labels.Set{rke2.MachineNameLabel: machine.Name}.AsSelector())
				if err != nil {
					return err
				}
				if len(planSecrets) == 0 {
					continue
				}

				for _, secret := range planSecrets {
					if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						secret, err := secretsController.Get(secret.Namespace, secret.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						secret = secret.DeepCopy()
						rke2.CopyMap(secret.Labels, machine.Labels)
						rke2.CopyMap(secret.Annotations, machine.Annotations)
						_, err = secretsController.Update(secret)
						return err
					}); err != nil {
						return err
					}
				}

				if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
					bootstrap, err := bootstrapClient.Get(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					bootstrap = bootstrap.DeepCopy()
					rke2.CopyMapWithExcludes(bootstrap.Labels, machine.Labels, bootstrapLabelExcludes)
					rke2.CopyMapWithExcludes(bootstrap.Annotations, machine.Annotations, boostrapAnnotationExcludes)
					if bootstrap.Spec.ClusterName == "" {
						// If the bootstrap spec cluster name is blank, we need to update the bootstrap spec to the correct value
						// This is to handle old rkebootstrap objects for unmanaged clusters that did not have the spec properly set
						if v, ok := bootstrap.Labels[capi.ClusterLabelName]; ok && v != "" {
							bootstrap.Spec.ClusterName = v
						}
					}
					_, err = bootstrapClient.Update(bootstrap)
					return err
				}); err != nil {
					return err
				}
			}
		}
	}

	cm.Data[capiMigratedKey] = "true"
	return createOrUpdateConfigMap(configMapController, cm)
}
