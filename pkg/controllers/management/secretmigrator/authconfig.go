package secretmigrator

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/objectclient"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/namespace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	serviceAccountPasswordFieldName = "serviceaccountpassword"
	authConfigKind                  = "authconfig"
)

// syncAuthConfig syncs the authentication config and removes/migrates secrets as needed.
func (h *handler) syncAuthConfig(_ string, authConfig *apimgmtv3.AuthConfig) (runtime.Object, error) {
	if authConfig == nil || !authConfig.Enabled || apimgmtv3.AuthConfigConditionSecretsMigrated.IsTrue(authConfig) {
		return authConfig, nil
	}

	if authConfig.Type != client.ShibbolethConfigType {
		apimgmtv3.AuthConfigConditionSecretsMigrated.SetStatus(authConfig, "True")
		updated, err := h.authConfigs.Update(authConfig.Name, authConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to update migration status of authconfig: %w", err)
		}
		return updated, nil
	}

	updated, err := apimgmtv3.AuthConfigConditionSecretsMigrated.DoUntilTrue(authConfig, func() (runtime.Object, error) {
		unstructuredConfig, err := getUnstructuredAuthConfig(h.authConfigs, authConfig)
		if err != nil {
			return nil, err
		}

		return h.migrateShibbolethSecrets(unstructuredConfig)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update status for AuthConfig %s: %w", authConfig.Name, err)
	}

	updatedAuthConfig, err := h.authConfigs.Update(authConfig.Name, updated)
	if err != nil {
		return nil, fmt.Errorf("failed to update AuthConfig %s: %w", authConfig.Name, err)
	}
	return updatedAuthConfig, nil
}

// getUnstructuredAuthConfig attempts to get the unstructured AuthConfig for the AuthConfig that is passed in.
func getUnstructuredAuthConfig(unstructuredClient objectclient.GenericClient, authConfig *apimgmtv3.AuthConfig) (map[string]any, error) {
	unstructuredAuthConfig, err := unstructuredClient.Get(authConfig.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve unstructured data for AuthConfig from cluster: %w", err)
	}

	unstructured, ok := unstructuredAuthConfig.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to read unstructured data for AuthConfig")
	}

	unstructuredConfig := unstructured.UnstructuredContent()
	return unstructuredConfig, nil
}

// migrateShibbolethSecrets effects the migration of secrets for the Shibboleth provider.
func (h *handler) migrateShibbolethSecrets(unstructuredConfig map[string]any) (runtime.Object, error) {
	shibbConfig := &apimgmtv3.ShibbolethConfig{}

	err := common.Decode(unstructuredConfig, shibbConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode ShibbolethConfig: %w", err)
	}

	if shibbConfig.OpenLdapConfig.ServiceAccountPassword == "" {
		// OpenLDAP is not configured, so nothing else is needed
		return shibbConfig, nil
	}

	secretName := fmt.Sprintf("%s-%s", strings.ToLower(shibbConfig.Type), serviceAccountPasswordFieldName)
	lowercaseFieldName := strings.ToLower(serviceAccountPasswordFieldName)

	// cannot use createOrUpdateSecretForCredential because the credential is saved in the secret with a key of
	// "credential", but our AuthProviders look for "serviceaccountpassword"
	_, err = h.migrator.createOrUpdateSecret(
		secretName,
		namespace.GlobalNamespace,
		map[string]string{
			lowercaseFieldName: shibbConfig.OpenLdapConfig.ServiceAccountPassword,
		},
		nil,
		shibbConfig,
		authConfigKind,
		lowercaseFieldName)
	if err != nil {
		return nil, err
	}

	lowerType := strings.ToLower(shibbConfig.Type)
	fullSecretName := common.GetFullSecretName(lowerType, serviceAccountPasswordFieldName)
	shibbConfig.OpenLdapConfig.ServiceAccountPassword = fullSecretName

	return shibbConfig, nil
}
