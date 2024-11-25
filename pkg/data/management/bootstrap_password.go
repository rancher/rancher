package management

import (
	"context"
	"github.com/sirupsen/logrus"
	"os"

	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	bootstrapPasswordSecretName = "bootstrap-secret"
	bootstrapPasswordSecretKey  = "bootstrapPassword"
)

// GetBootstrapPassword reads the bootstrap password from its secret.  If the secret is not found
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

	if s != nil {
		// if the bootstrap password is set return it
		bootstrapPasswordBytes, hasPasswordKey := s.Data[bootstrapPasswordSecretKey]
		if hasPasswordKey {
			return string(bootstrapPasswordBytes), generated, nil
		}
		logrus.Warn("A bootstrap password secret was found, but did not match the expected structure.")
	}

	// if the password secret is not set check the env for user-input, or fall back to generating one
	userPasswordValue := os.Getenv("CATTLE_BOOTSTRAP_PASSWORD")
	s.StringData = make(map[string]string)
	if userPasswordValue == "" {
		generated = true
		userPasswordValue, _ = randomtoken.Generate()
	}
	s.StringData[bootstrapPasswordSecretKey] = userPasswordValue

	// persist the password
	if s != nil && s.GetResourceVersion() != "" {
		_, err = secrets.Update(ctx, s, metav1.UpdateOptions{})
	} else {
		s.Name = bootstrapPasswordSecretName
		_, err = secrets.Create(ctx, s, metav1.CreateOptions{})
	}
	if err != nil {
		return "", false, err
	}

	return userPasswordValue, generated, nil
}
