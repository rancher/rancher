package rancher

import (
	"github.com/mcuadros/go-version"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rancherversion "github.com/rancher/rancher/pkg/version"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
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

	// we only want to migrate if the previous installation was earlier than the migration version
	if lastMigration, ok := cm.Data[rancherVersionKey]; ok {
		if lastMigration == "dev" || version.Compare(migrationVersion, lastMigration, "<=") {
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
