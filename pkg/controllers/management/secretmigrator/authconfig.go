package secretmigrator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/namespace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	serviceAccountPasswordFieldName = "serviceaccountpassword"
	authConfigKind                  = "authconfig"
)

// syncAuthConfig syncs the authentication config and removes/migrates secrets as needed.
func (h *handler) syncAuthConfig(_ string, authConfig *apimgmtv3.AuthConfig) (runtime.Object, error) {
	if authConfig == nil || !authConfig.Enabled {
		return authConfig, nil
	}

	switch authConfig.Type {
	case client.ShibbolethConfigType:
		obj, err := h.migrateAuthConfigPasswordToSecret(authConfig,
			apimgmtv3.AuthConfigConditionSecretsMigrated, h.migrateShibbolethSecrets)
		if err != nil {
			return obj, err
		}
		return h.fixShibbolethSecretReference(obj)
	case client.OKTAConfigType:
		return h.migrateAuthConfigPasswordToSecret(authConfig,
			apimgmtv3.AuthConfigOKTAPasswordMigrated,
			h.migrateOKTASecrets)
	default:
		return h.migrateAuthConfig(authConfig)
	}
}

func (h *handler) migrateAuthConfig(authConfig *apimgmtv3.AuthConfig) (runtime.Object, error) {
	unstructuredConfig, err := getUnstructuredAuthConfigByName(h.authConfigs, authConfig.Name)
	if err != nil {
		return nil, err
	}
	newUnstructuredConfig, err := setUnstructuredStatus(unstructuredConfig, apimgmtv3.AuthConfigConditionSecretsMigrated, "True")
	if err != nil {
		return nil, fmt.Errorf("failed to set the status on unstructured AuthConfig %s: %w", authConfig.Name, err)
	}

	updated, err := h.authConfigs.Update(authConfig.Name, newUnstructuredConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to update migration status of authconfig: %w", err)
	}
	return updated, nil
}

func (h *handler) migrateAuthConfigPasswordToSecret(authConfig *apimgmtv3.AuthConfig, cond condition.Cond, f func(runtime.Unstructured) (runtime.Object, error)) (runtime.Object, error) {
	if cond.IsTrue(authConfig) {
		return authConfig, nil
	}

	updated, err := cond.DoUntilTrue(authConfig, func() (runtime.Object, error) {
		unstructuredConfig, err := getUnstructuredAuthConfigByName(h.authConfigs, authConfig.Name)
		if err != nil {
			return nil, err
		}

		return f(unstructuredConfig)
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

func (h *handler) fixShibbolethSecretReference(obj runtime.Object) (runtime.Object, error) {
	shibbConfig := &apimgmtv3.ShibbolethConfig{}
	switch v := obj.(type) {
	case *apimgmtv3.ShibbolethConfig:
		shibbConfig = v
	case *apimgmtv3.AuthConfig:
		unstructuredConfig, err := getUnstructuredAuthConfigByName(h.authConfigs, v.Name)
		if err != nil {
			return nil, err
		}

		if err := common.Decode(unstructuredConfig.UnstructuredContent(), shibbConfig); err != nil {
			return nil, fmt.Errorf("unable to decode ShibbolethConfig: %w", err)
		}
	}

	if apimgmtv3.AuthConfigConditionShibbolethSecretFixed.IsTrue(shibbConfig) {
		return obj, nil
	}

	updated, err := apimgmtv3.AuthConfigConditionShibbolethSecretFixed.DoUntilTrue(shibbConfig, func() (runtime.Object, error) {
		// This is fixing a bug where the secret was created as a lower-case
		// name, but referenced with mixed-case.
		const badSecretName = "cattle-global-data:shibbolethconfig-serviceAccountPassword"
		if shibbConfig.OpenLdapConfig.ServiceAccountPassword == badSecretName {
			shibbConfig.OpenLdapConfig.ServiceAccountPassword = strings.ToLower(shibbConfig.OpenLdapConfig.ServiceAccountPassword)
		}
		return shibbConfig, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update status for AuthConfig %s: %w", shibbConfig.Name, err)
	}

	updatedShibbolethConfig, err := h.authConfigs.Update(shibbConfig.Name, updated)
	if err != nil {
		return nil, fmt.Errorf("failed to update AuthConfig %s: %w", shibbConfig.Name, err)
	}

	return updatedShibbolethConfig, nil
}

// migrateShibbolethSecrets effects the migration of secrets for the Shibboleth provider.
func (h *handler) migrateShibbolethSecrets(unstructuredConfig runtime.Unstructured) (runtime.Object, error) {
	shibbConfig := &apimgmtv3.ShibbolethConfig{}
	err := common.Decode(unstructuredConfig.UnstructuredContent(), shibbConfig)
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
	secret, err := h.migrator.createOrUpdateSecret(
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

	shibbConfig.OpenLdapConfig.ServiceAccountPassword = common.NameForSecret(secret)

	return shibbConfig, nil
}

// migrateOKTASecrets effects the migration of secrets for the OKTA provider.
func (h *handler) migrateOKTASecrets(unstructuredConfig runtime.Unstructured) (runtime.Object, error) {
	oktaConfig := &apimgmtv3.OKTAConfig{}
	err := common.Decode(unstructuredConfig.UnstructuredContent(), oktaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode OKTAConfig: %w", err)
	}

	if oktaConfig.OpenLdapConfig.ServiceAccountPassword == "" {
		// OpenLDAP is not configured, so nothing else is needed
		return oktaConfig, nil
	}
	secretName := fmt.Sprintf("%s-%s", strings.ToLower(oktaConfig.Type), serviceAccountPasswordFieldName)
	lowercaseFieldName := strings.ToLower(serviceAccountPasswordFieldName)

	// cannot use createOrUpdateSecretForCredential because the credential is saved in the secret with a key of
	// "credential", but our AuthProviders look for "serviceaccountpassword"
	_, err = h.migrator.createOrUpdateSecret(
		secretName,
		namespace.GlobalNamespace,
		map[string]string{
			lowercaseFieldName: oktaConfig.OpenLdapConfig.ServiceAccountPassword,
		},
		nil,
		oktaConfig,
		authConfigKind,
		lowercaseFieldName)
	if err != nil {
		return nil, err
	}

	lowerType := strings.ToLower(oktaConfig.Type)
	fullSecretName := common.GetFullSecretName(lowerType, serviceAccountPasswordFieldName)
	oktaConfig.OpenLdapConfig.ServiceAccountPassword = fullSecretName

	return oktaConfig, nil
}

func setUnstructuredStatus(unstructured runtime.Unstructured, key condition.Cond, value corev1.ConditionStatus) (runtime.Unstructured, error) {
	content := unstructured.UnstructuredContent()
	status, ok := content["status"].(map[string]any)
	if !ok {
		status = map[string]any{}
	}

	var authConfigStatus apimgmtv3.AuthConfigStatus
	if err := mapstructure.Decode(status, &authConfigStatus); err != nil {
		return nil, err
	}
	var found bool
	for i, cond := range authConfigStatus.Conditions {
		if cond.Type == key {
			authConfigStatus.Conditions[i].Status = value
			found = true
			break
		}
	}
	if !found {
		authConfigStatus.Conditions = append(authConfigStatus.Conditions, apimgmtv3.AuthConfigConditions{
			Type:   key,
			Status: value,
		})
	}
	newBytes, err := json.Marshal(authConfigStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal auth config status to bytes %w", err)
	}
	var newContent map[string]any
	if err := json.Unmarshal(newBytes, &newContent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth config status as bytes to map %w", err)
	}
	content["status"] = newContent

	unstructured.SetUnstructuredContent(content)
	return unstructured, nil
}

// getUnstructuredAuthConfig attempts to get the unstructured AuthConfig for the AuthConfig that is passed in.
func getUnstructuredAuthConfigByName(unstructuredClient authConfigsClient, name string) (runtime.Unstructured, error) {
	unstructuredAuthConfig, err := unstructuredClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve unstructured data for AuthConfig from cluster: %w", err)
	}

	unstructured, ok := unstructuredAuthConfig.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to read unstructured data for AuthConfig")
	}

	return unstructured, nil
}
