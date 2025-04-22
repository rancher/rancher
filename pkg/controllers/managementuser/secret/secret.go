package secret

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretController listens for secret CUD in management API
// and propagates the changes to all corresponding namespaces in cluster API

// NamespaceController listens to cluster namespace events,
// reads secrets from the management namespace of corresponding project,
// and creates the secrets in the cluster namespace

const (
	create                     = "create"
	update                     = "update"
	projectNamespaceAnnotation = "management.cattle.io/system-namespace"
	userSecretAnnotation       = "secret.user.cattle.io/secret"

	syncAnnotation             = "provisioning.cattle.io/sync"
	syncPreBootstrapAnnotation = "provisioning.cattle.io/sync-bootstrap"
	syncNamespaceAnnotation    = "provisioning.cattle.io/sync-target-namespace"
	syncNameAnnotation         = "provisioning.cattle.io/sync-target-name"
	syncedAtAnnotation         = "provisioning.cattle.io/synced-at"
)

var (
	// as of right now kube-system is the only appropriate target for this functionality.
	// also included is "" to support copying into the same ns the secret is in
	approvedPreBootstrapTargetNamespaces = []string{"kube-system", ""}
)

type ResourceSyncController struct {
	upstreamSecrets   v1.SecretInterface
	downstreamSecrets v1.SecretInterface
	clusterName       string
	clusterId         string
}

func Bootstrap(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster) error {
	c := &ResourceSyncController{
		upstreamSecrets:   mgmt.Core.Secrets(clusterRec.Spec.FleetWorkspaceName),
		downstreamSecrets: cluster.Core.Secrets(""),
		clusterName:       clusterRec.Spec.DisplayName,
		clusterId:         clusterRec.Name,
	}

	return c.bootstrap(mgmt.Management.Clusters(""), clusterRec)
}

func Register(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster) {
	register(ctx, cluster)

	resourceSyncController := &ResourceSyncController{
		upstreamSecrets:   mgmt.Core.Secrets(clusterRec.Spec.FleetWorkspaceName),
		downstreamSecrets: cluster.Core.Secrets(""),
		clusterName:       clusterRec.Spec.DisplayName,
		clusterId:         clusterRec.Name,
	}

	resourceSyncController.upstreamSecrets.AddHandler(ctx, "secret-resource-synced", resourceSyncController.sync)
}

func (c *ResourceSyncController) bootstrap(mgmtClusterClient v3.ClusterInterface, mgmtCluster *apimgmtv3.Cluster) error {
	secrets, err := c.upstreamSecrets.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list secrets in %v namespace: %w", mgmtCluster.Spec.FleetWorkspaceName, err)
	}

	logrus.Debugf("[pre-bootstrap][secrets] looking for secrets-to synchronize to cluster %v", c.clusterName)

	for _, sec := range secrets.Items {
		s := &sec

		if !c.bootstrapSyncable(s) {
			continue
		}

		logrus.Debugf("[pre-bootstrap-sync][secrets] syncing secret %v/%v to cluster %v", s.Namespace, s.Name, c.clusterName)

		_, err = c.sync("", s)
		if err != nil {
			return fmt.Errorf("failed to synchronize secret %v/%v to cluster %v: %w", s.Namespace, s.Name, c.clusterName, err)
		}

		logrus.Debugf("[pre-boostrap-sync][secret] successfully synced secret %v/%v to downstream cluster %v", s.Namespace, s.Name, c.clusterName)
	}

	apimgmtv3.ClusterConditionPreBootstrapped.True(mgmtCluster)
	_, err = mgmtClusterClient.Update(mgmtCluster)
	if err != nil {
		return fmt.Errorf("failed to update cluster bootstrap condition for %v: %w", c.clusterName, err)
	}

	return nil
}

func (c *ResourceSyncController) syncable(obj *corev1.Secret) bool {
	// no sync annotations, we don't care about this secret
	if obj.Annotations[syncAnnotation] == "" && obj.Annotations[syncPreBootstrapAnnotation] == "" {
		return false
	}

	// if secret is authorized to be synchronized to the cluster
	if !slices.Contains(strings.Split(obj.Annotations[capr.AuthorizedObjectAnnotation], ","), c.clusterName) {
		return false
	}

	// if the secret is not in a namespace that we are allowed to sync to
	if !slices.Contains(approvedPreBootstrapTargetNamespaces, obj.Annotations[syncNamespaceAnnotation]) {
		return false
	}

	return true
}

func (c *ResourceSyncController) bootstrapSyncable(obj *corev1.Secret) bool {
	// only difference between sync and bootstrapSync is requiring the boostrap sync annotation to be set to "true"
	return c.syncable(obj) && obj.Annotations[syncPreBootstrapAnnotation] == "true"
}

func (c *ResourceSyncController) injectClusterIdIntoSecretData(sec *corev1.Secret) *corev1.Secret {
	for key, value := range sec.Data {
		if bytes.Contains(value, []byte("{{clusterId}}")) {
			sec.Data[key] = bytes.ReplaceAll(value, []byte("{{clusterId}}"), []byte(c.clusterId))
		}
	}

	return sec
}

func (c *ResourceSyncController) removeClusterIdFromSecretData(sec *corev1.Secret) *corev1.Secret {
	for key, value := range sec.Data {
		if bytes.Contains(value, []byte(c.clusterId)) {
			sec.Data[key] = bytes.ReplaceAll(value, []byte(c.clusterId), []byte("{{clusterId}}"))
		}
	}

	return sec
}

func (c *ResourceSyncController) sync(key string, obj *corev1.Secret) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	if !c.syncable(obj) {
		return obj, nil
	}

	name := obj.Annotations[syncNameAnnotation]
	if name == "" {
		name = obj.Name
	}
	ns := obj.Annotations[syncNamespaceAnnotation]
	if ns == "" {
		ns = obj.Namespace
	}

	logrus.Debugf("[resource-sync][secret] synchronizing %v/%v to %v/%v for cluster %v", obj.Namespace, obj.Name, ns, name, c.clusterName)

	var targetSecret *corev1.Secret
	var err error
	if targetSecret, err = c.downstreamSecrets.GetNamespaced(ns, name, metav1.GetOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get downstream secret %v/%v in cluster %v: %w", ns, name, c.clusterName, err)
	}

	if targetSecret == nil || errors.IsNotFound(err) {
		logrus.Debugf("[resource-sync][secret] creating secret %v/%v in cluster %v", ns, name, c.clusterName)

		newSecret := &corev1.Secret{
			Type:       obj.Type,
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Data:       obj.Data,
		}

		newSecret = c.injectClusterIdIntoSecretData(newSecret)

		_, err = c.downstreamSecrets.Create(newSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to create secret %v/%v in cluster %v: %w", ns, name, c.clusterName, err)
		}
	} else if !reflect.DeepEqual(c.removeClusterIdFromSecretData(targetSecret).Data, obj.Data) {
		logrus.Debugf("[resource-sync][secret] updating secret %v/%v in cluster %v", ns, name, c.clusterName)

		targetSecret.Data = obj.Data
		targetSecret = c.injectClusterIdIntoSecretData(targetSecret)

		_, err = c.downstreamSecrets.Update(targetSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret %v/%v in cluster %v: %w", ns, name, c.clusterName, err)
		}
	} else {
		logrus.Debugf("[resource-sync][secret] skipping downstream update - contents are the same")
		return obj, nil
	}

	logrus.Debugf("[resource-sync][secret] successfully synchronized secret %v/%v to %v/%v for cluster %v", obj.Namespace, obj.Name, ns, name, c.clusterName)

	obj.Annotations[syncedAtAnnotation] = time.Now().Format(time.RFC3339)
	return obj, nil
}
