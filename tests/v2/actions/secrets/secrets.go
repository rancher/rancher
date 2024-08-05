package secrets

import (
	"fmt"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/secrets"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
)

// CreateSecret is a helper to create a secret using wrangler client
func CreateSecret(client *rancher.Client, clusterID, namespaceName string, data map[string][]byte) (*corev1.Secret, error) {
	var ctx *wrangler.Context
	var err error

	if clusterID != "local" {
		ctx, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to get downstream context: %w", err)
		}
	} else {
		ctx = client.WranglerContext
	}

	secretName := namegen.AppendRandomString("testsecret")
	secretTemplate := secrets.NewSecretTemplate(secretName, namespaceName, data, corev1.SecretTypeOpaque)

	createdSecret, err := ctx.Core.Secret().Create(&secretTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	return createdSecret, nil
}

// UpdateSecretData is a helper to update the existing secret data with new  data
func UpdateSecretData(secret *corev1.Secret, newData map[string][]byte) *corev1.Secret {
	updatedSecretObj := secret.DeepCopy()
	if updatedSecretObj.Data == nil {
		updatedSecretObj.Data = make(map[string][]byte)
	}

	for key, value := range newData {
		updatedSecretObj.Data[key] = value
	}

	return updatedSecretObj
}
