package management

import (
	"context"
	"os"

	"github.com/rancher/wrangler/v2/pkg/randomtoken"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	bootstrapPasswordSecretName = "bootstrap-secret"
	bootstrapPasswordSecretKey  = "bootstrapPassword"
)

// GetBootstrapPassword reads the bootstrap password from it's secret.  If the secret is not found
// it will be set from `CATTLE_BOOTSTRAP_PASSWORD` or generated if this is empty as well.
func GetBootstrapPassword(ctx context.Context, secrets corev1.SecretInterface) (string, bool, error) {
	var s *v1.Secret
	var err error
	var generated bool

	// load the existing secret
	s, err = secrets.Get(ctx, bootstrapPasswordSecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", false, err
	}

	// if the bootstrap password is set return it
	if s.StringData[bootstrapPasswordSecretKey] != "" {
		return s.StringData[bootstrapPasswordSecretKey], generated, nil
	}

	// if the password is not set check the env and fall back to generating one
	s.StringData = make(map[string]string)
	s.StringData[bootstrapPasswordSecretKey] = os.Getenv("CATTLE_BOOTSTRAP_PASSWORD")
	if s.StringData[bootstrapPasswordSecretKey] == "" {
		generated = true
		s.StringData[bootstrapPasswordSecretKey], _ = randomtoken.Generate()
	}

	// persist the password
	if s.GetResourceVersion() != "" {
		_, err = secrets.Update(ctx, s, metav1.UpdateOptions{})
	} else {
		s.Name = bootstrapPasswordSecretName
		_, err = secrets.Create(ctx, s, metav1.CreateOptions{})
	}
	if err != nil {
		return "", false, err
	}

	return s.StringData[bootstrapPasswordSecretKey], generated, nil
}
