package management

import (
	"context"
	"os"

	"github.com/rancher/wrangler/pkg/randomtoken"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	bootstrapPasswordSecretName = "bootstrap-secret"
	bootstrapPasswordSecretKey  = "bootstrapPassword"
)

// GetBootstrapPassword queries the corresponding bootstrap-secret in the cattle-system namespace. If this secret
// exists, and has the corresponding bootstrapPassword key, then the value of that key will be returned. Otherwise, the
// `CATTLE_BOOTSTRAP_PASSWORD` environment variable will be checked. If neither are set, a bootstrap password is
// generated.
func GetBootstrapPassword(ctx context.Context, secrets corev1.SecretInterface) (string, bool, error) {
	var generated bool

	// load the existing secret
	secret, err := secrets.Get(ctx, bootstrapPasswordSecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", false, err
	}

	// if the bootstrap password is set return it
	if password, ok := secret.Data[bootstrapPasswordSecretKey]; ok {
		return string(password), generated, nil
	}

	// if the password is not set check the env and fall back to generating one
	secret.StringData = make(map[string]string)
	secret.StringData[bootstrapPasswordSecretKey] = os.Getenv("CATTLE_BOOTSTRAP_PASSWORD")
	if secret.StringData[bootstrapPasswordSecretKey] == "" {
		generated = true
		secret.StringData[bootstrapPasswordSecretKey], _ = randomtoken.Generate()
	}

	// persist the password
	if secret.GetResourceVersion() != "" {
		_, err = secrets.Update(ctx, secret, metav1.UpdateOptions{})
	} else {
		secret.Name = bootstrapPasswordSecretName
		_, err = secrets.Create(ctx, secret, metav1.CreateOptions{})
	}
	if err != nil {
		return "", false, err
	}

	return secret.StringData[bootstrapPasswordSecretKey], generated, nil
}
