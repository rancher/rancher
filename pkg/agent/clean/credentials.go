package clean

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/pagination"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// UnusedPullSecrets removes rancher managed image pull secrets
// from the cattle-system namespace that are no longer used by the cattle-cluster-agent.
// This can occur in imported or hosted clusters, when either the cluster or global pull secret
// configuration is changed after initial provisioning.
func UnusedPullSecrets(ctx context.Context) {
	client, err := rest.InClusterConfig()
	if err != nil {
		logrus.Errorf("Error getting in cluster config: %v", err)
		return
	}

	k8s, err := kubernetes.NewForConfig(client)
	if err != nil {
		logrus.Errorf("Error getting kubernetes client: %v", err)
		return
	}
	unusedPullSecrets(ctx, k8s.CoreV1().Secrets("cattle-system"), strings.Split(os.Getenv("CATTLE_AGENT_PULL_SECRETS"), ","))
}

func unusedPullSecrets(ctx context.Context, secret typedcorev1.SecretInterface, configuredPullSecrets []string) {
	var secretsToRemove []corev1.Secret
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", cluster.AgentPullSecretLabel),
	}
	listPullSecrets := func(opts metav1.ListOptions) (*corev1.SecretList, error) {
		return secret.List(ctx, opts)
	}
	processPullSecrets := func(copiedPullSecrets *corev1.SecretList) error {
		for _, pullSecret := range copiedPullSecrets.Items {
			if !slices.Contains(configuredPullSecrets, pullSecret.Name) {
				secretsToRemove = append(secretsToRemove, pullSecret)
			}
		}
		return nil
	}
	err := pagination.EachPage(listPullSecrets, listOpts, processPullSecrets)
	if err != nil {
		logrus.Errorf("Error listing cattle-cluster-agent-pull-secrets: %v", err)
		return
	}

	if len(secretsToRemove) == 0 {
		return
	}

	logrus.Infof("removing %d unused pull secrets", len(secretsToRemove))
	for _, pullSecret := range secretsToRemove {
		err = secret.Delete(ctx, pullSecret.Name, metav1.DeleteOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logrus.Errorf("Error deleting pull secret: %v", err)
			}
			continue
		}
		logrus.Infof("removed unused pull secret %q", pullSecret.Name)
	}

	logrus.Infof("removed %d unused pull secrets", len(secretsToRemove))
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

	return removeUnusedCattleCredentialsFrom(ctx, k8s.CoreV1().Secrets("cattle-system"))
}

func removeUnusedCattleCredentialsFrom(ctx context.Context, s typedcorev1.SecretInterface) error {
	listSecrets := func(opts metav1.ListOptions) (*corev1.SecretList, error) {
		return s.List(ctx, opts)
	}
	processSecrets := func(secrets *corev1.SecretList) error {
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
			err := s.Delete(ctx, sec.Name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
	return pagination.EachPage(listSecrets, metav1.ListOptions{}, processSecrets)
}
