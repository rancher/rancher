package secretmigrator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/objectclient"
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
	if authConfig == nil || !authConfig.Enabled || apimgmtv3.AuthConfigConditionSecretsMigrated.IsTrue(authConfig) {
		return authConfig, nil
	}

	if authConfig.Type != client.ShibbolethConfigType {
		unstructuredConfig, err := getUnstructuredAuthConfig(h.authConfigs, authConfig)
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

	updated, err := apimgmtv3.AuthConfigConditionSecretsMigrated.DoUntilTrue(authConfig, func() (runtime.Object, error) {
		unstructuredConfig, err := getUnstructuredAuthConfig(h.authConfigs, authConfig)
		if err != nil {
			return nil, err
		}

		return h.migrateShibbolethSecrets(unstructuredConfig.UnstructuredContent())
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
func getUnstructuredAuthConfig(unstructuredClient objectclient.GenericClient, authConfig *apimgmtv3.AuthConfig) (runtime.Unstructured, error) {
	unstructuredAuthConfig, err := unstructuredClient.Get(authConfig.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve unstructured data for AuthConfig from cluster: %w", err)
	}

	unstructured, ok := unstructuredAuthConfig.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to read unstructured data for AuthConfig")
	}
	return unstructured, nil
}

// migrateShibbolethSecrets effects the migration of secrets for the Shibboleth provider.
func (h *handler) migrateShibbolethSecrets(unstructuredConfig map[string]any) (*apimgmtv3.ShibbolethConfig, error) {
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
