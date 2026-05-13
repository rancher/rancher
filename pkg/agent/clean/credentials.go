package clean

import (
	"context"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// UnusedPullSecrets removes rancher managed image pull secrets
// from the cattle-system namespace that are no longer used by the cattle-cluster-agent.
// This can occur in imported or hosted clusters, when either the cluster or global pull secret
// configuration is changed after initial provisioning.
func UnusedPullSecrets(ctx context.Context) error {
	client, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	k8s, err := kubernetes.NewForConfig(client)
	if err != nil {
		return err
	}
	secret := k8s.CoreV1().Secrets("cattle-system")
	copiedPullSecrets, err := secret.List(ctx, metav1.ListOptions{
		LabelSelector: "management.cattle.io/cattle-cluster-agent-pull-secret=true",
	})
	if err != nil {
		return err
	}

	configuredPullSecrets := strings.Split(os.Getenv("CATTLE_AGENT_PULL_SECRETS"), ",")

	var secretsToRemove []corev1.Secret
	for _, pullSecret := range copiedPullSecrets.Items {
		if !slices.Contains(configuredPullSecrets, pullSecret.Name) {
			secretsToRemove = append(secretsToRemove, pullSecret)
		}
	}

	if len(secretsToRemove) == 0 {
		return nil
	}

	logrus.Infof("removing %d unused pull secrets", len(secretsToRemove))
	for _, pullSecret := range secretsToRemove {
		err = secret.Delete(ctx, pullSecret.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		logrus.Infof("removed unused pull secret %q", pullSecret.Name)
	}

	logrus.Infof("removed %d unused pull secrets", len(secretsToRemove))
	return nil
}

// UnusedCattleCredentials removes all of the cattle-credentials that aren't being used by the current pod on a ticker. Meant to be run from a goroutine so it doesn't stop the parent execution
func UnusedCattleCredentials() {
	for range time.Tick(time.Hour) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		err := removeUnusedCattleCredentials(ctx)
		if err != nil {
			logrus.Errorf("Error removing unused cattle credentials: %v", err)
		}
		cancel()
	}
}

// removeUnusedCattleCredentials goes through all the secrets in the cattle-system namespace and removes any that are
// not being used by the current pod and are older than an hour. This prevents any leftover tokens from being left
// in the cluster on rotation. The main reason we wait an hour is just in case the deployment is updated and/or
// flapping it won't nuke the secret unless it's been unused for a little while.
func removeUnusedCattleCredentials(ctx context.Context) error {
	client, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	k8s, err := kubernetes.NewForConfig(client)
	if err != nil {
		return err
	}

	s := k8s.CoreV1().Secrets("cattle-system")

	secrets, err := s.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, sec := range secrets.Items {
		if !strings.HasPrefix(sec.Name, "cattle-credentials-") {
			continue
		}

		if sec.Name == os.Getenv("CATTLE_CREDENTIAL_NAME") {
			continue
		}

		if time.Since(sec.CreationTimestamp.Time) < time.Hour {
			continue
		}

		logrus.Infof("Deleting unused cattle-credentials secret: %s", sec.Name)
		err = s.Delete(ctx, sec.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
