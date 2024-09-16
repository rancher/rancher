package secret

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/norman/controller"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// SecretController listens for secret CUD in management API
// and propagates the changes to all corresponding namespaces in cluster API

// NamespaceController listens to cluster namespace events,
// reads secrets from the management namespace of corresponding project,
// and creates the secrets in the cluster namespace

const (
	projectIDLabel             = "field.cattle.io/projectId"
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

type Controller struct {
	secrets                   v1.SecretInterface
	clusterNamespaceLister    v1.NamespaceLister
	managementNamespaceLister v1.NamespaceLister
	projectLister             v3.ProjectLister
	clusterName               string
}

type DirectSyncController struct {
	upstreamSecrets   v1.SecretInterface
	downstreamSecrets v1.SecretInterface
	clusterName       string
}

func Bootstrap(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster) error {
	dsc := &DirectSyncController{
		upstreamSecrets:   mgmt.Core.Secrets("fleet-default"),
		downstreamSecrets: cluster.Core.Secrets(""),
		clusterName:       clusterRec.Spec.DisplayName,
	}

	return dsc.bootstrap(mgmt.Management.Clusters(""), clusterRec)
}

func Register(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, cluster)
		return nil
	})

	projectLister := cluster.Management.Management.Projects("").Controller().Lister()
	secrets := cluster.Management.Core.Secrets("")
	secrets.AddHandler(ctx, "secret-deferred", func(key string, obj *corev1.Secret) (runtime.Object, error) {
		if obj == nil {
			return nil, nil
		}

		_, err := projectLister.Get(cluster.ClusterName, obj.Namespace)
		if errors.IsNotFound(err) {
			return obj, nil
		} else if err != nil {
			return obj, err
		}

		if !strings.HasPrefix(obj.Name, "default-token-") {
			return obj, starter()
		}

		return obj, nil
	})

	directSyncController := &DirectSyncController{
		upstreamSecrets:   mgmt.Core.Secrets("fleet-default"),
		downstreamSecrets: cluster.Core.Secrets(""),
		clusterName:       clusterRec.Spec.DisplayName,
	}

	directSyncController.upstreamSecrets.AddHandler(ctx, "secret-direct-synced", directSyncController.sync)
}

func (dsc *DirectSyncController) bootstrap(mgmtClusterClient v3.ClusterInterface, mgmtCluster *apimgmtv3.Cluster) error {
	secrets, err := dsc.upstreamSecrets.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list secrets in fleet-default namespace: %v", err)
	}

	logrus.Debugf("[pre-bootstrap][secrets] looking for secrets-to synchronize to cluster %v", dsc.clusterName)

	for _, sec := range secrets.Items {
		if !dsc.bootstrapSyncable(&sec) {
			continue
		}

		logrus.Debugf("[pre-bootstrap-sync][secrets] syncing secret %v/%v to cluster %v", sec.Namespace, sec.Name, dsc.clusterName)

		_, err := dsc.sync("", &sec)
		if err != nil {
			return fmt.Errorf("[pre-bootstrap][secret] failed to synchronize secret %v/%v to cluster %v: %w", sec.Namespace, sec.Name, dsc.clusterName, err)
		}

		logrus.Debugf("[pre-boostrap-sync][secret] successfully synced secret %v/%v to downstream cluster %v", sec.Namespace, sec.Name, dsc.clusterName)
	}

	apimgmtv3.ClusterConditionBootstrapped.True(mgmtCluster)
	_, err = mgmtClusterClient.Update(mgmtCluster)
	if err != nil {
		return fmt.Errorf("failed to update cluster bootstrap condition for %v: %v", dsc.clusterName, err)
	}

	return nil
}

func (dsc *DirectSyncController) syncable(obj *corev1.Secret) bool {
	// no sync annotations, we don't care about this secret
	if obj.Annotations[syncAnnotation] == "" && obj.Annotations[syncPreBootstrapAnnotation] == "" {
		return false
	}

	// if secret is authorized to be synchronized to the cluster
	if !slices.Contains(strings.Split(obj.Annotations[capr.AuthorizedObjectAnnotation], ","), dsc.clusterName) {
		return false
	}

	// if the secret is not in a namespace that we are allowed to sync to
	if !slices.Contains(approvedPreBootstrapTargetNamespaces, obj.Annotations[syncNamespaceAnnotation]) {
		return false
	}

	return true
}

func (dsc *DirectSyncController) bootstrapSyncable(obj *corev1.Secret) bool {
	// only difference between sync and bootstrapSync is requiring the boostrap sync annotation to be set to "true"
	return dsc.syncable(obj) && obj.Annotations[syncPreBootstrapAnnotation] == "true"
}

func (dsc *DirectSyncController) sync(key string, obj *corev1.Secret) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	if !dsc.syncable(obj) {
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

	logrus.Debugf("[direct-sync][secret] synchronizing %v/%v to %v/%v for cluster %v", obj.Namespace, obj.Name, ns, name, dsc.clusterName)

	op := "update"
	// fetching every time due to the fact that we don't have a downstream cache in the upstream cluster anymore
	targetSecret, err := dsc.downstreamSecrets.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Debugf("[direct-sync][secret] secret %v/%v not found for for cluster %v, creating", ns, name, dsc.clusterName)
			op = "create"
		} else {
			return nil, fmt.Errorf("failed to get downstream secret %v/%v in cluster %v: %v", ns, name, dsc.clusterName, err)
		}
	}

	if targetSecret != nil && reflect.DeepEqual(targetSecret.Data, obj.Data) {
		logrus.Debugf("[direct-sync][secret] skipping downstream update - contents are the same")
		return obj, nil
	}

	if op == "create" {
		logrus.Debugf("[direct-sync][secret] creating secret %v/%v in cluster %v", ns, name, dsc.clusterName)

		targetSecret, err = dsc.downstreamSecrets.Create(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Data:       obj.Data,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create secret %v/%v in cluster %v: %v", ns, name, dsc.clusterName, err)
		}
	} else {
		logrus.Debugf("[direct-sync][secret] updating secret %v/%v in cluster %v", ns, name, dsc.clusterName)

		targetSecret.Data = obj.Data
		_, err = dsc.downstreamSecrets.Update(targetSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret %v/%v in cluster %v: %v", ns, name, dsc.clusterName, err)
		}
	}

	logrus.Debugf("[direct-sync][secret] successfully synchronized secret %v/%v to %v/%v for cluster %v", obj.Namespace, obj.Name, ns, name, dsc.clusterName)

	obj.Annotations[syncedAtAnnotation] = time.Now().Format(time.RFC3339)
	return obj, nil
}

func registerDeferred(ctx context.Context, cluster *config.UserContext) {
	clusterSecretsClient := cluster.Core.Secrets("")
	s := &Controller{
		secrets:                   clusterSecretsClient,
		clusterNamespaceLister:    cluster.Core.Namespaces("").Controller().Lister(),
		managementNamespaceLister: cluster.Management.Core.Namespaces("").Controller().Lister(),
		projectLister:             cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		clusterName:               cluster.ClusterName,
	}

	n := &NamespaceController{
		clusterSecretsClient: clusterSecretsClient,
		managementSecrets:    cluster.Management.Core.Secrets("").Controller().Lister(),
	}
	cluster.Core.Namespaces("").AddHandler(ctx, "secretsController", n.sync)

	sync := v1.NewSecretLifecycleAdapter(fmt.Sprintf("secretsController_%s", cluster.ClusterName), true,
		cluster.Management.Core.Secrets(""), s)

	cluster.Management.Core.Secrets("").AddHandler(ctx, "secretsController", func(key string, obj *corev1.Secret) (runtime.Object, error) {
		if obj == nil {
			logrus.Tracef("secretsController: AddHandler: obj is nil, calling sync")
			return sync(key, nil)
		}
		if !controller.ObjectInCluster(cluster.ClusterName, obj) {
			logrus.Tracef("secretsController: AddHandler: obj [%s] is not in cluster [%s], returning nil", obj.Name, cluster.ClusterName)
			return nil, nil
		}

		if obj.Type == corev1.SecretTypeServiceAccountToken {
			logrus.Tracef("secretsController: AddHandler: obj [%s] is Service Account token, skipping", obj.Name)
			return nil, nil
		}

		if obj.Labels != nil {
			if obj.Labels["cattle.io/creator"] == "norman" {
				logrus.Tracef("secretsController: AddHandler: obj [%s] labels in [%s] contain cattle.io/creator=norman, calling sync", obj.Name, cluster.ClusterName)
				return sync(key, obj)
			}
		}

		return nil, nil
	})
}

type NamespaceController struct {
	clusterSecretsClient v1.SecretInterface
	managementSecrets    v1.SecretLister
}

func (n *NamespaceController) sync(key string, obj *corev1.Namespace) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Tracef("secretsController: sync called for key [%s] in namespace [%s]", key, obj.Name)
	// field.cattle.io/projectId value is <cluster name>:<project name>
	logrus.Tracef("secretsController: sync: key [%s], obj.Annotations[projectIDLabel]: [%s]", key, obj.Annotations[projectIDLabel])
	if obj.Annotations[projectIDLabel] != "" {
		parts := strings.Split(obj.Annotations[projectIDLabel], ":")
		if len(parts) == 2 {
			if parts[1] == "" {
				logrus.Debugf("[NamspaceController|sync] empty project name found in obj.Annotations[projectIDLabel] for cluster: %s", parts[0])
				return nil, nil
			}
			// on the management side, secret's namespace name equals to project name
			secrets, err := n.managementSecrets.List(parts[1], labels.NewSelector())
			if err != nil {
				return nil, err
			}
			logrus.Tracef("secretsController: sync: length of secrets for [%s] in namespace [%s] is %d", parts[1], obj.Name, len(secrets))
			for _, secret := range secrets {
				// skip service account token secrets
				if secret.Type == corev1.SecretTypeServiceAccountToken {
					logrus.Tracef("secretsController: AddHandler: secret [%s] is Service Account token, skipping", secret.Name)
					continue
				}
				namespacedSecret := getNamespacedSecret(secret, obj.Name)
				if _, err := n.clusterSecretsClient.GetNamespaced(namespacedSecret.Namespace, namespacedSecret.Name, metav1.GetOptions{}); err == nil {
					continue
				}
				logrus.Infof("Creating secret [%s] into namespace [%s]", namespacedSecret.Name, obj.Name)
				_, err := n.clusterSecretsClient.Create(namespacedSecret)
				if err != nil && !errors.IsAlreadyExists(err) {
					return nil, err
				}
			}
		}
	}
	return nil, nil
}

func (s *Controller) Create(obj *corev1.Secret) (runtime.Object, error) {
	logrus.Tracef("secretsController: Create called for [%s]", obj.Name)
	return nil, s.createOrUpdate(obj, create)
}

func (s *Controller) Updated(obj *corev1.Secret) (runtime.Object, error) {
	logrus.Tracef("secretsController: Updated called for [%s]", obj.Name)
	return nil, s.createOrUpdate(obj, update)
}

func (s *Controller) Remove(obj *corev1.Secret) (runtime.Object, error) {
	logrus.Tracef("secretsController: Remove called for [%s]", obj.Name)
	clusterNamespaces, err := s.getClusterNamespaces(obj)
	if err != nil {
		return nil, err
	}

	for _, namespace := range clusterNamespaces {
		logrus.Infof("Deleting secret [%s] in namespace [%s]", obj.Name, namespace.Name)
		if err := s.secrets.DeleteNamespaced(namespace.Name, obj.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
	}
	return nil, nil
}

func (s *Controller) getClusterNamespaces(obj *corev1.Secret) ([]*corev1.Namespace, error) {
	logrus.Tracef("secretsController: getClusterNamespaces called for [%s]", obj.Name)
	var toReturn []*corev1.Namespace
	projectNamespace, err := s.managementNamespaceLister.Get("", obj.Namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Warnf("Project namespace [%s] can't be found", obj.Namespace)
			return toReturn, nil
		}
		return toReturn, err
	}
	if projectNamespace.Annotations == nil {
		return toReturn, nil
	}
	if val, ok := projectNamespace.Annotations[projectNamespaceAnnotation]; !(ok && val == "true") {
		return toReturn, nil
	}

	// Ignore projects from other clusters. Project namespace name = project name, so use it to locate the project
	_, err = s.projectLister.Get(s.clusterName, projectNamespace.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return toReturn, nil
		}
		return toReturn, err
	}

	namespaces, err := s.clusterNamespaceLister.List("", labels.NewSelector())
	if err != nil {
		return toReturn, err
	}
	// system project namespace name == project.Name
	projectID := obj.Namespace

	for _, namespace := range namespaces {
		parts := strings.Split(namespace.Annotations[projectIDLabel], ":")
		if len(parts) == 2 && parts[1] == projectID {
			toReturn = append(toReturn, namespace)
		}
	}
	return toReturn, nil
}

func (s *Controller) createOrUpdate(obj *corev1.Secret, action string) error {
	logrus.Tracef("secretsController: createOrUpdate called for [%s]", obj.Name)
	if obj.Annotations[projectIDLabel] != "" {
		parts := strings.Split(obj.Annotations[projectIDLabel], ":")
		if len(parts) == 2 {
			if parts[0] != s.clusterName {
				return nil
			}
		}
	}
	clusterNamespaces, err := s.getClusterNamespaces(obj)
	if err != nil {
		return err
	}
	for _, namespace := range clusterNamespaces {
		if !namespace.DeletionTimestamp.IsZero() {
			continue
		}
		// copy the secret into namespace
		namespacedSecret := getNamespacedSecret(obj, namespace.Name)
		switch action {
		case create:
			if _, err := s.secrets.GetNamespaced(namespacedSecret.Namespace, namespacedSecret.Name, metav1.GetOptions{}); err == nil {
				continue
			}
			logrus.Infof("Copying secret [%s] into namespace [%s]", namespacedSecret.Name, namespace.Name)
			_, err := s.secrets.Create(namespacedSecret)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		case update:
			if existing, err := s.secrets.GetNamespaced(namespacedSecret.Namespace, namespacedSecret.Name, metav1.GetOptions{}); err == nil &&
				reflect.DeepEqual(existing.Data, namespacedSecret.Data) {
				continue
			}
			logrus.Infof("Updating secret [%s] into namespace [%s]", namespacedSecret.Name, namespace.Name)
			_, err := s.secrets.Update(namespacedSecret)
			if err != nil && !errors.IsNotFound(err) {
				return err
			} else if errors.IsNotFound(err) {
				logrus.Infof("Updating secret [%s] returned NotFound, creating secret [%s] into namespace [%s]", namespacedSecret.Name, namespacedSecret.Name, namespace.Name)
				_, err := s.secrets.Create(namespacedSecret)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func getNamespacedSecret(obj *corev1.Secret, namespace string) *corev1.Secret {
	namespacedSecret := &corev1.Secret{}
	namespacedSecret.Name = obj.Name
	namespacedSecret.Kind = obj.Kind
	namespacedSecret.Data = obj.Data
	namespacedSecret.StringData = obj.StringData
	namespacedSecret.Namespace = namespace
	namespacedSecret.Type = obj.Type
	namespacedSecret.Annotations = make(map[string]string)
	namespacedSecret.Labels = make(map[string]string)
	copyMap(namespacedSecret.Annotations, obj.Annotations)
	copyMap(namespacedSecret.Labels, obj.Labels)
	namespacedSecret.Annotations[userSecretAnnotation] = "true"
	return namespacedSecret
}

func copyMap(dst map[string]string, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}
