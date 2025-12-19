package secrets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	clusterapi "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/clusters"
	"github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	secretsapi "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/secrets"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	SecretSteveType                   = "secret"
	ProjectScopedSecretLabel          = "management.cattle.io/project-scoped-secret"
	ProjectScopedSecretCopyAnnotation = "management.cattle.io/project-scoped-secret-copy"
)

// CreateSecret is a helper to create a secret using wrangler client
func CreateSecret(client *rancher.Client, clusterID, namespaceName string, data map[string][]byte, secretType corev1.SecretType, labels, annotations map[string]string) (*corev1.Secret, error) {
	ctx, err := clusterapi.GetClusterWranglerContext(client, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster context: %w", err)
	}

	if labels == nil {
		labels = make(map[string]string)
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}

	secretName := namegen.AppendRandomString("testsecret")
	secretTemplate := NewSecretTemplate(secretName, namespaceName, data, secretType, labels, annotations)

	createdSecret, err := ctx.Core.Secret().Create(&secretTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	return createdSecret, nil
}

// SecretCopyWithNewData is a helper to create a copy of an existing secret with new data.
func SecretCopyWithNewData(secret *corev1.Secret, newData map[string][]byte) *corev1.Secret {
	updatedSecretObj := secret.DeepCopy()
	if updatedSecretObj.Data == nil {
		updatedSecretObj.Data = make(map[string][]byte)
	}

	for key, value := range newData {
		updatedSecretObj.Data[key] = value
	}

	return updatedSecretObj
}

// CreateRegistrySecretDockerConfigJSON is a helper to generate dockerconfigjson content for a registry secret
func CreateRegistrySecretDockerConfigJSON(registryconfig *Config) (string, error) {
	registry := registryconfig.Name
	username := registryconfig.Username
	password := registryconfig.Password

	if username == "" || password == "" {
		return "", fmt.Errorf("missing registry credentials in the config file")
	}

	auth := map[string]interface{}{
		"username": username,
		"password": password,
		"auth":     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
	}

	config := map[string]interface{}{
		"auths": map[string]interface{}{
			registry: auth,
		},
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(configJSON), nil
}

// CreateProjectScopedSecret creates a project-scoped secret in the project's backing namespace in the local cluster
func CreateProjectScopedSecret(client *rancher.Client, clusterID, projectID string, data map[string][]byte, secretType corev1.SecretType) (*corev1.Secret, error) {
	backingNamespace := fmt.Sprintf("%s-%s", clusterID, projectID)

	labels := map[string]string{
		ProjectScopedSecretLabel: projectID,
	}

	createdProjectScopedSecret, err := CreateSecret(client, rbac.LocalCluster, backingNamespace, data, secretType, labels, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create project scoped secret: %w", err)
	}

	return createdProjectScopedSecret, nil
}

// ValidatePropagatedNamespaceSecrets verifies that secrets propagated to project namespaces match the original project-scoped secret
func ValidatePropagatedNamespaceSecrets(client *rancher.Client, clusterID, projectID string, projectScopedSecret *corev1.Secret, namespaceList []*corev1.Namespace) error {
	return kwait.PollUntilContextTimeout(context.TODO(), defaults.FiveSecondTimeout, defaults.TenSecondTimeout, true, func(ctx context.Context) (bool, error) {
		for _, ns := range namespaceList {
			nsSecret, err := secretsapi.GetSecretByName(client, clusterID, ns.Name, projectScopedSecret.Name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}

			if nsSecret.Labels[ProjectScopedSecretLabel] != projectID {
				return false, nil
			}

			if nsSecret.Annotations == nil || nsSecret.Annotations[ProjectScopedSecretCopyAnnotation] != "true" {
				return false, nil
			}

			if !reflect.DeepEqual(projectScopedSecret.Data, nsSecret.Data) {
				return false, nil
			}
		}
		return true, nil
	})
}

// ValidateProjectScopedSecretLabel ensures the project-scoped secret has the correct label
func ValidateProjectScopedSecretLabel(projectScopedSecret *corev1.Secret, expectedProjectID string) error {
	actualLabel, labelExists := projectScopedSecret.Labels[ProjectScopedSecretLabel]
	if !labelExists || actualLabel != expectedProjectID {
		return fmt.Errorf("project scoped secret missing or incorrect label '%s=%s'", ProjectScopedSecretLabel, expectedProjectID)
	}
	return nil
}

// UpdateSecretData updates the data of a secret in the specified namespace using the wrangler client
func UpdateSecretData(client *rancher.Client, clusterID, namespace, secretName string, newData map[string][]byte) (*corev1.Secret, error) {
	ctx, err := clusterapi.GetClusterWranglerContext(client, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster context: %w", err)
	}

	existingSecret, err := ctx.Core.Secret().Get(namespace, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	existingSecret.Data = newData
	updatedSecret, err := ctx.Core.Secret().Update(existingSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to update secret %s: %w", secretName, err)
	}

	return updatedSecret, nil
}

// UpdateProjectScopedSecret updates the data of an project-scoped secret in the backing namespace of a project
func UpdateProjectScopedSecret(client *rancher.Client, clusterID, projectID, secretName string, newData map[string][]byte) (*corev1.Secret, error) {
	backingNamespace := fmt.Sprintf("%s-%s", clusterID, projectID)

	updatedProjectScopedSecret, err := UpdateSecretData(client, rbac.LocalCluster, backingNamespace, secretName, newData)
	if err != nil {
		return nil, fmt.Errorf("failed to update secret %s: %w", secretName, err)
	}

	return updatedProjectScopedSecret, nil
}

// DeleteSecret deletes a secret from a specific namespace in the given cluster using the wrangler client
func DeleteSecret(client *rancher.Client, clusterID, namespaceName, secretName string) error {
	ctx, err := clusterapi.GetClusterWranglerContext(client, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster context: %w", err)
	}

	return ctx.Core.Secret().Delete(namespaceName, secretName, &metav1.DeleteOptions{})
}

// WaitForSecretInNamespaces waits for a secret to either exist or be deleted in the given list of namespaces.
// If shouldExist is true, it waits for the secret to be present in all namespaces.
// If shouldExist is false, it waits for the secret to be absent from all namespaces.
func WaitForSecretInNamespaces(client *rancher.Client, clusterID, secretName string, namespaceList []*corev1.Namespace, shouldExist bool) error {
	for _, ns := range namespaceList {
		err := kwait.PollUntilContextTimeout(context.TODO(), defaults.FiveSecondTimeout, defaults.TwoMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
			_, err = secretsapi.GetSecretByName(client, clusterID, ns.Name, secretName, metav1.GetOptions{})
			if shouldExist {
				if err != nil {
					if apierrors.IsNotFound(err) {
						return false, nil
					}
					return false, err
				}
				return true, nil
			}

			if err == nil {
				return false, nil
			}
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err

		})
		if err != nil {
			return fmt.Errorf("waiting for secret %s in namespace %s failed: %w", secretName, ns.Name, err)
		}
	}
	return nil
}
