package certsexpiration

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/client-go/kubernetes"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

const getStateTimeout = time.Second * 30

const (
	FullStateConfigMapName = "full-cluster-state"
	FullStateSecretName    = "full-cluster-state"
)

type FullState struct {
	DesiredState State `json:"desiredState,omitempty"`
	CurrentState State `json:"currentState,omitempty"`
}

type State struct {
	CertificatesBundle map[string]rkecerts.CertificatePKI `json:"certificatesBundle,omitempty"`
}

// This controller handles cert expiration for local cluster only
func Register(ctx context.Context, management *config.ManagementContext) {
	c := &certsExpiration{
		clusters:  management.Management.Clusters(""),
		k8sClient: management.K8sClient,
	}
	management.Management.Clusters("").AddHandler(ctx, "certificate-expiration", c.sync)
}

type certsExpiration struct {
	clusters  v3.ClusterInterface
	k8sClient kubernetes.Interface
}

func (c *certsExpiration) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.Name != "local" {
		return cluster, nil // We are only checking local cluster
	}
	fullState, err := getFullStateFromK8s(context.Background(), c.k8sClient)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return cluster, nil // not an rke cluster, nothing we can do
		}
		return cluster, err
	}

	certBundle := fullState.CurrentState.CertificatesBundle
	rkecerts.CleanCertificateBundle(certBundle)

	certsExpInfo := map[string]v32.CertExpiration{}
	for certName, certObj := range certBundle {
		info, err := rkecerts.GetCertExpiration(certObj.CertificatePEM)
		if err != nil {
			logrus.Debugf("failed to get expiration date for certificate [%s] for local cluster: %v", certName, err)
			continue
		}
		certsExpInfo[certName] = info
		err = logCertExpirationWarning(certName, info)
		if err != nil {
			logrus.Warnf("certificate [%s] from local cluster has or will expire and date is corrupted: %v", certName, err)
			continue
		}
	}
	// Update certExpiration on cluster obj in order for it to display in API, and the UI if expiring
	if !reflect.DeepEqual(cluster.Status.CertificatesExpiration, certsExpInfo) {
		cluster.Status.CertificatesExpiration = certsExpInfo
		return c.clusters.Update(cluster)
	}
	return cluster, nil
}

func logCertExpirationWarning(name string, certExp v32.CertExpiration) error {
	date, err := time.Parse(time.RFC3339, certExp.ExpirationDate)
	if err != nil {
		return err
	}
	if time.Now().UTC().After(date) { // warn if expired
		logrus.Warnf("Certificate from local cluster has expired: %s", name)
	} else if time.Now().UTC().AddDate(0, 1, 0).After(date) { // warn if within a month
		logrus.Warnf("Certificate from local cluster will expire soon: %s", name)
	}
	return nil
}

// getFullStateFromK8s fetches the full cluster state from the k8s cluster.
// In earlier versions of RKE, the full cluster state was stored in a configmap, but it has since been moved
// to a secret. This function tries fetching it from the secret first and will fall back on the configmap if the secret
// doesn't exist.
func getFullStateFromK8s(ctx context.Context, k8sClient kubernetes.Interface) (*FullState, error) {
	// Back off for 1s between attempts.
	backoff := wait.Backoff{
		Duration: time.Second,
		Steps:    int(getStateTimeout.Seconds()),
	}

	// Try to fetch secret or configmap in k8s.
	var fullState FullState
	getState := func(ctx context.Context) (bool, error) {
		fullStateBytes, err := getFullStateBytesFromSecret(ctx, k8sClient, FullStateSecretName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Debug("full-state secret not found, falling back to configmap")

				fullStateBytes, err = getFullStateBytesFromConfigMap(ctx, k8sClient, FullStateConfigMapName)
				if err != nil {
					return false, fmt.Errorf("[state] error getting full state from configmap: %w", err)
				}
			} else {
				return false, fmt.Errorf("[state] error getting full state from secret: %w", err)
			}
		}

		if err := json.Unmarshal(fullStateBytes, &fullState); err != nil {
			return false, fmt.Errorf("[state] error unmarshalling full state from JSON: %w", err)
		}

		return true, nil
	}

	// Retry until success or backoff.Steps has been reached or ctx is cancelled.
	err := wait.ExponentialBackoffWithContext(ctx, backoff, getState)
	return &fullState, err
}

// getFullStateBytesFromConfigMap fetches the full state from the configmap with the given name in the kube-system namespace.
func getFullStateBytesFromConfigMap(ctx context.Context, k8sClient kubernetes.Interface, name string) ([]byte, error) {
	confMap, err := k8sClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("[state] error getting configmap %s: %w", name, err)
	}

	data, ok := confMap.Data[name]
	if !ok {
		return nil, fmt.Errorf("[state] expected configmap %s to have field %s, but none was found", name, name)
	}

	return []byte(data), nil
}

// getFullStateBytesFromSecret fetches the full state from the secret with the given name in the kube-system namespace.
func getFullStateBytesFromSecret(ctx context.Context, k8sClient kubernetes.Interface, name string) ([]byte, error) {
	secret, err := k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("[state] error getting secret %s: %w", name, err)
	}

	data, ok := secret.Data[name]
	if !ok {
		return nil, fmt.Errorf("[state] expected secret %s to have field %s, but none was found", name, name)
	}

	return data, nil
}
