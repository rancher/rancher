package resourcesyncer

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/utils/strings/slices"

	corecontrollers "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// resourceSyncer synchronizes secrets between upstream and downstream clusters.
type resourceSyncer struct {
	upstreamSecretClient   corecontrollers.SecretInterface
	downstreamSecretClient corecontrollers.SecretInterface

	mgmtClusterName string
	rkeControlPlane *types.NamespacedName
}

const (
	SyncToCluster   = "provisioning.cattle.io/sync-to-cluster"
	SyncToNamespace = "provisioning.cattle.io/sync-to-namespace"
	SyncedTimestamp = "provisioning.cattle.io/last-synced-at"
)

// list of namespaces that are allowed to have resources synchronized between
// the upstream and downstream cluster
var allowedNamespaces = []string{"kube-system"}

func Register(ctx context.Context, cluster *config.UserContext, mgmt *config.ScaledContext) {
	// we only want to run this controller for non-local clusters
	if cluster.ClusterName == "local" {
		return
	}

	r := resourceSyncer{
		upstreamSecretClient:   mgmt.Core.Secrets(""),
		downstreamSecretClient: cluster.Core.Secrets(""),
		mgmtClusterName:        cluster.ClusterName,
	}

	// populate the rkeControlPlane field so that it's available in the handler
	controlPlanes, err := mgmt.Wrangler.RKE.RKEControlPlane().Cache().List("", labels.Everything())
	if err != nil {
		logrus.Warnf("failed to list RKEControlPlanes from cache: %v", err)
		return
	}

	for _, plane := range controlPlanes {
		if plane.Spec.ManagementClusterName == r.mgmtClusterName {
			r.rkeControlPlane = &types.NamespacedName{
				Name:      plane.Name,
				Namespace: plane.Namespace,
			}
			break
		}
	}

	if r.rkeControlPlane == nil {
		logrus.Warnf("failed to find RKEControlPlane for cluster %v", r.mgmtClusterName)
		return
	}

	for _, ns := range allowedNamespaces {
		// TODO: all resources we want synced will need to register this handler (probably just secrets/configmaps for now)
		mgmt.Core.Secrets(ns).Controller().AddHandler(ctx, "secret-sync", r.syncSecret)
	}
}

func (r *resourceSyncer) syncSecret(key string, s *v1.Secret) (runtime.Object, error) {
	if s == nil {
		return nil, nil
	}

	// no sync annotation -> ignore it
	if s.Annotations[SyncToCluster] == "" {
		return s, nil
	}

	// make sure we're syncing to the proper cluster
	if s.Annotations[SyncToCluster] != r.rkeControlPlane.Name {
		// not syncing it downstream since it isn't in the same cluster
		return s, nil
	}

	ns := s.Annotations[SyncToNamespace]
	if ns == "" {
		ns = s.Namespace
	}

	// ns annotation -> sync to that namespace if allowed
	if !slices.Contains(allowedNamespaces, ns) {
		logrus.Warnf("[resource-sync] secret %s/%s is not in the allowed namespaces list, ignoring it", ns, s.Name)
		return s, nil
	}

	obj, err := r.createOrUpdateSecretDownstream(ns, s)
	if err != nil {
		logrus.Errorf("[resource-sync] error syncing secret %s/%s: %v", ns, s.Name, err)
	}

	return obj, err
}

func (r *resourceSyncer) createOrUpdateSecretDownstream(ns string, upstreamSecret *v1.Secret) (runtime.Object, error) {
	logrus.Debugf("[resource-sync] syncing secret %s/%s to cluster %s", ns, upstreamSecret.Name, r.rkeControlPlane.Name)

	// first try to fetch the secret
	ds, err := r.downstreamSecretClient.GetNamespaced(ns, upstreamSecret.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get downstream secret: %w", err)
	} else if errors.IsNotFound(err) {
		// otherwise create the secret
		_, err = r.downstreamSecretClient.Create(
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      upstreamSecret.Name,
					Namespace: ns,
				},
				Data: upstreamSecret.Data,
			},
		)
		if err != nil {
			return upstreamSecret, fmt.Errorf("failed to create secret downstream: %w", err)
		}
	} else if reflect.DeepEqual(upstreamSecret.Data, ds.Data) {
		logrus.Debugf("[resource-sync] secret %s/%s is already up to date in downstream namespace", ns, upstreamSecret.Name)
		return upstreamSecret, nil
	} else {
		ds.Data = upstreamSecret.Data
		_, err = r.downstreamSecretClient.Update(ds)
		if err != nil {
			return upstreamSecret, fmt.Errorf("failed to update secret downstream: %w", err)
		}
	}

	// store the updated applied timestamp only after we create/update it downstream successfully
	logrus.Debugf("[resource-sync] successfully synced secret %s/%s to cluster %s", ns, upstreamSecret.Name, r.rkeControlPlane.Name)
	upstreamSecret.Annotations[SyncedTimestamp] = time.Now().Format(time.RFC3339)
	return r.upstreamSecretClient.Update(upstreamSecret)
}
