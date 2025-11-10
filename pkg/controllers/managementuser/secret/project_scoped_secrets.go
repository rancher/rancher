package secret

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	userSecretAnnotation     = "secret.user.cattle.io/secret"
	namespaceChangeHandler   = "project-scoped-secret-namespace-handler"
	namespaceEnqueuerName    = "project-scoped-secret-namespace-enqueuer"
	projectIDLabel           = "field.cattle.io/projectId"
	ProjectScopedSecretLabel = "management.cattle.io/project-scoped-secret"
	pssCopyAnnotation        = "management.cattle.io/project-scoped-secret-copy"
)

// previous controller label, annotations and finalizer
const (
	normanCreatorLabel = "cattle.io/creator"
	oldPSSAnnotation   = "lifecycle.cattle.io/create.secretsController_"
	oldPSSFinalizer    = "clusterscoped.controller.cattle.io/secretsController_"
)

type namespaceHandler struct {
	managementSecretCache  wcorev1.SecretCache
	managementSecretClient wcorev1.SecretClient
	clusterNamespaceCache  wcorev1.NamespaceCache
	projectCache           mgmtv3.ProjectCache
	secretClient           wcorev1.SecretClient
	clusterName            string
}

func RegisterProjectScopedSecretHandler(ctx context.Context, cluster *config.UserContext) {
	n := &namespaceHandler{
		secretClient:           cluster.Corew.Secret(),
		managementSecretCache:  cluster.Management.Wrangler.Core.Secret().Cache(),
		managementSecretClient: cluster.Management.Wrangler.Core.Secret(),
		projectCache:           cluster.Management.Wrangler.Mgmt.Project().Cache(),
		clusterNamespaceCache:  cluster.Corew.Namespace().Cache(),
		clusterName:            cluster.ClusterName,
	}
	cluster.Corew.Namespace().OnChange(ctx, namespaceChangeHandler, n.OnChange)
	relatedresource.WatchClusterScoped(ctx, namespaceEnqueuerName, n.secretEnqueueNamespace, cluster.Corew.Namespace(), cluster.Management.Wrangler.Core.Secret())
}

func (n *namespaceHandler) OnChange(_ string, namespace *corev1.Namespace) (*corev1.Namespace, error) {
	if namespace == nil || namespace.DeletionTimestamp != nil {
		return nil, nil
	}

	project, err := n.getProjectFromNamespace(namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting project for namespace %s: %w", namespace.Name, err)
	}
	if project == nil {
		return nil, nil
	}

	// migrate existing project scoped secrets
	if err := n.migrateExistingProjectScopedSecrets(project); err != nil {
		return nil, err
	}

	secrets, err := n.getProjectScopedSecretsFromNamespace(project)
	if err != nil {
		return nil, err
	}

	var errs error
	desiredSecrets := sets.New[types.NamespacedName]()

	// create/update project scoped secrets
	for _, secret := range secrets {
		secretCopy := getNamespacedSecret(secret, namespace.Name)

		err := rbac.CreateOrUpdateNamespacedResource(secretCopy, n.secretClient, areSecretsSame)
		desiredSecrets.Insert(client.ObjectKeyFromObject(secretCopy))
		errs = errors.Join(errs, err)
	}
	if errs != nil {
		return nil, errs
	}

	return namespace, n.removeUndesiredProjectScopedSecrets(namespace, desiredSecrets)
}

// migrateExistingProjectScopedSecrets migrates existing project scoped secrets.
// It removes the following:
//   - Finalizer "clusterscoped.controller.cattle.io/secretsController_<clusterID>"
//   - Annotation "lifecycle.cattle.io/create.secretsController_<clusterID>"
//   - Label "rancher.io/creator"
//
// And adds:
//   - Label "management.cattle.io/project-scoped-secret"
func (n *namespaceHandler) migrateExistingProjectScopedSecrets(project *v3.Project) error {
	backingNamespace := project.GetProjectBackingNamespace()
	clusterName := project.Spec.ClusterName

	r, err := labels.NewRequirement(normanCreatorLabel, selection.Equals, []string{"norman"})
	if err != nil {
		return err
	}
	secrets, err := n.managementSecretCache.List(backingNamespace, labels.NewSelector().Add(*r))
	if err != nil {
		return err
	}

	// Remove the norman creator label, secret lifecycle annotation and secret lifecycle finalizer.
	// The controllers handling those were removed in v2.12 https://github.com/rancher/rancher/pull/49995
	var errs error
	for _, s := range secrets {
		secretCopy := s.DeepCopy()
		delete(secretCopy.Labels, normanCreatorLabel)
		delete(secretCopy.Annotations, oldPSSAnnotation+clusterName)
		if i := slices.Index(secretCopy.Finalizers, oldPSSFinalizer+clusterName); i >= 0 {
			secretCopy.Finalizers = slices.Delete(secretCopy.Finalizers, i, i+1)
		}
		secretCopy.Labels[ProjectScopedSecretLabel] = project.Name
		_, err := n.managementSecretClient.Update(secretCopy)
		errs = errors.Join(errs, err)
	}

	return errs
}

// getProjectScopedSecretsFromNamespace gets all project scoped secret from a project namespace.
func (n *namespaceHandler) getProjectScopedSecretsFromNamespace(project *v3.Project) ([]*corev1.Secret, error) {
	backingNamespace := project.GetProjectBackingNamespace()

	r, err := labels.NewRequirement(ProjectScopedSecretLabel, selection.Equals, []string{project.Name})
	if err != nil {
		return nil, err
	}
	return n.managementSecretCache.List(backingNamespace, labels.NewSelector().Add(*r))
}

// removeUndesiredProjectScopedSecrets removes project scoped secrets from the namespace that are not in the desiredSecrets map.
func (n *namespaceHandler) removeUndesiredProjectScopedSecrets(namespace *corev1.Namespace, desiredSecrets sets.Set[types.NamespacedName]) error {
	// remove any project scoped secrets that don't belong in the project
	downstreamProjectScopedSecrets, err := n.secretClient.List(namespace.Name, metav1.ListOptions{
		LabelSelector: ProjectScopedSecretLabel,
	})
	if err != nil {
		return err
	}

	allSecrets := sets.New[types.NamespacedName]()
	for _, secret := range downstreamProjectScopedSecrets.Items {
		// only remove secrets that are copies
		if secret.Annotations[pssCopyAnnotation] == "true" {
			allSecrets.Insert(client.ObjectKeyFromObject(&secret))
		}
	}

	secretsToDelete := allSecrets.Difference(desiredSecrets)

	var errs error
	for _, secret := range secretsToDelete.UnsortedList() {
		// secret in namespace does not belong here
		logrus.Infof("Cleaning project scoped secret %s from namespace %s", secret.Name, secret.Namespace)
		errs = errors.Join(errs, n.secretClient.Delete(namespace.Name, secret.Name, &metav1.DeleteOptions{}))
	}
	return errs
}

// getProjectFromNamespace returns the project that a namespace belongs to, if it belongs to one.
// Returns nil if the namespace is not part of a project or if the projectID annotation is malformed.
func (n *namespaceHandler) getProjectFromNamespace(namespace *corev1.Namespace) (*v3.Project, error) {
	// check if namespace is part of a project
	projectID, ok := namespace.Annotations[projectIDLabel]
	if !ok {
		return nil, nil
	}
	clusterName, projectName, found := strings.Cut(projectID, ":")
	if !found {
		logrus.Debugf("Namespace %s projectId annotation %s is malformed, should be <cluster name>:<project name>", namespace.Name, namespace.Annotations[projectIDLabel])
		return nil, nil
	}

	project, err := n.projectCache.Get(clusterName, projectName)
	if apierrors.IsNotFound(err) {
		logrus.Warnf("Namespace %s references project %s:%s which does not exist. Not re-enqueueing", namespace.Name, clusterName, projectName)
		return nil, nil
	}
	return project, err
}

// secretEnqueueNamespace enqueues all the project namespaces of a project scoped secret.
func (n *namespaceHandler) secretEnqueueNamespace(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a secret", obj)
		return nil, nil
	}

	namespaces, err := n.getNamespacesFromSecret(secret)
	if err != nil {
		return nil, err
	}

	namespaceKeys := make([]relatedresource.Key, 0, len(namespaces))
	for _, namespace := range namespaces {
		namespaceKeys = append(namespaceKeys, relatedresource.Key{Name: namespace.Name})
	}
	return namespaceKeys, nil
}

// getNamespacesFromSecret returns a slice of project namespaces from a project scoped secret.
func (n *namespaceHandler) getNamespacesFromSecret(secret *corev1.Secret) ([]*corev1.Namespace, error) {
	if secret.Labels == nil {
		return nil, nil
	}

	// we only care about project scoped secrets
	projectName, ok := secret.Labels[ProjectScopedSecretLabel]
	if !ok {
		return nil, nil
	}
	project, err := n.projectCache.Get(n.clusterName, projectName)
	if apierrors.IsNotFound(err) {
		// this controller is called for every cluster, so if the project isn't found, it's likely a project in a different cluster
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if project.GetProjectBackingNamespace() != secret.Namespace {
		logrus.Tracef("Secret [%s] not in the project namespace, not copying", secret.Name)
		return nil, nil
	}

	r, err := labels.NewRequirement(projectIDLabel, selection.Equals, []string{project.Name})
	if err != nil {
		return nil, err
	}
	return n.clusterNamespaceCache.List(labels.NewSelector().Add(*r))
}

// getNamespacedSecret copies a project scoped secret and replaces the namespace with the passed in namespace.
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
	maps.Copy(namespacedSecret.Annotations, obj.Annotations)
	maps.Copy(namespacedSecret.Labels, obj.Labels)
	namespacedSecret.Annotations[userSecretAnnotation] = "true"
	namespacedSecret.Annotations[pssCopyAnnotation] = "true"
	return namespacedSecret
}

func areSecretsSame(s1, s2 *corev1.Secret) (bool, *corev1.Secret) {
	return reflect.DeepEqual(s1.Data, s2.Data) &&
		s1.Annotations[ProjectScopedSecretLabel] == s2.Annotations[ProjectScopedSecretLabel] &&
		s1.Annotations[pssCopyAnnotation] == s2.Annotations[pssCopyAnnotation], s2
}
