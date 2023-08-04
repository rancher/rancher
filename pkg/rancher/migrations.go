package rancher

import (
	"fmt"
	"strings"

	"github.com/mcuadros/go-version"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	cattleNamespace          = "cattle-system"
	forceUpgradeLogoutConfig = "forceupgradelogout"
	rancherVersionKey        = "rancherVersion"
)

func migrateCAPIKubeconfigs(w *wrangler.Context) error {
	logrus.Info("Running CAPI secret migration")

	namespaces, err := w.Core.Namespace().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing namespaces: %w", err)
	}

	for _, ns := range namespaces.Items {
		secrets, err := w.Core.Secret().List(ns.Name, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("listing secrets in namespace %s: %w", ns.Name, err)
		}

		for _, secret := range secrets.Items {
			if !isRKE2KubeConfigSecret(&secret) {
				logrus.Tracef("secret %s/%s is not a kubeconfig secret", ns.Name, secret.Name)
				continue
			}

			_, ok := secret.Labels[kubeconfig.ClusterLabelName]
			if ok {
				logrus.Tracef("kubeconfig secret %s/%s already has the capi cluster label", ns.Name, secret.Name)
				continue
			}

			clusterName := getRKE2ClusterName(&secret)
			if clusterName == "" {
				logrus.Tracef("kubeconfig secret %s/%s is not owned by a RKE2 cluster", ns.Name, secret.Name)
				continue
			}

			secretCopy := secret.DeepCopy()
			secretCopy.Labels[kubeconfig.ClusterLabelName] = clusterName

			if _, updateErr := w.Core.Secret().Update(secretCopy); updateErr != nil {
				return fmt.Errorf("updating secret %s/%s to add capi label: %w", ns.Name, secret.Name, err)
			}

			logrus.Debugf("Updated kubeconfig secret %s/%s with CAPI cluster label", ns.Name, secret.Name)
		}
	}

	return nil
}

// forceUpgradeLogout will delete all dashboard tokens forcing a logout.  This is useful when there is a major frontend
// upgrade and we want all users to be sent to a central point.  This function will check for the `forceUpgradeLogoutConfig`
// configuration map and only run if the last migrated version is lower than the given `migrationVersion`.
func forceUpgradeLogout(configMapController controllerv1.ConfigMapController, tokenController v3.TokenController, migrationVersion string) error {
	cm, err := configMapController.Cache().Get(cattleNamespace, forceUpgradeLogoutConfig)
	if err != nil && !k8serror.IsNotFound(err) {
		return err
	}

	// if this is the first ever migration initialize the configmap
	if cm == nil {
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      forceUpgradeLogoutConfig,
				Namespace: cattleNamespace,
			},
			Data: make(map[string]string, 1),
		}
	}

	// we do not migrate in development environments
	if rancherversion.Version == "dev" {
		return nil
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

	// record the migration being completed
	cm.Data[rancherVersionKey] = rancherversion.Version
	if cm.ObjectMeta.GetResourceVersion() != "" {
		_, err = configMapController.Update(cm)
	} else {
		_, err = configMapController.Create(cm)
	}

	return err
}

func isRKE2KubeConfigSecret(secret *v1.Secret) bool {
	if !strings.HasSuffix(secret.Name, "-kubeconfig") {
		return false
	}

	if len(secret.OwnerReferences) == 0 {
		return false
	}

	for _, ref := range secret.OwnerReferences {
		if ref.Kind == rkev1.SchemeGroupVersion.Identifier() && ref.Kind == "Cluster" {
			return true
		}
	}

	return false
}

func getRKE2ClusterName(secret *v1.Secret) string {
	if len(secret.OwnerReferences) == 0 {
		return ""
	}

	for _, ref := range secret.OwnerReferences {
		if ref.Kind == rkev1.SchemeGroupVersion.Identifier() && ref.Kind == "Cluster" {
			return ref.Name
		}
	}

	return ""
}
