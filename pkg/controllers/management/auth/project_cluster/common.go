package project_cluster

import (
	"context"
	"fmt"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	CreatorIDAnnotation             = "field.cattle.io/creatorId"
	NoCreatorRBACAnnotation         = "field.cattle.io/no-creator-rbac"
	creatorPrincipalNameAnnotation  = "field.cattle.io/creator-principal-name"
	creatorOwnerBindingAnnotation   = "authz.management.cattle.io/creator-owner-binding"
	roleTemplatesRequiredAnnotation = "authz.management.cattle.io/creator-role-bindings"
)

var crtbCreatorOwnerAnnotations = map[string]string{creatorOwnerBindingAnnotation: "true"}

func deleteNamespace(controller string, nsName string, nsClient v1.NamespaceInterface) error {
	ns, err := nsClient.Get(context.TODO(), nsName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	if ns.Status.Phase != v12.NamespaceTerminating {
		logrus.Infof("[%s] Deleting namespace %s", controller, nsName)
		err = nsClient.Delete(context.TODO(), nsName, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func reconcileResourceToNamespace(obj runtime.Object, controller string, nsName string, nsLister corev1.NamespaceLister, nsClient v1.NamespaceInterface) (runtime.Object, error) {
	return apisv3.NamespaceBackedResource.Do(obj, func() (runtime.Object, error) {
		t, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		ns, _ := nsLister.Get("", nsName)
		if ns == nil {
			logrus.Infof("[%v] Creating namespace %v", controller, nsName)
			_, err := nsClient.Create(context.TODO(), &v12.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsName,
					Annotations: map[string]string{
						"management.cattle.io/system-namespace": "true",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return obj, fmt.Errorf("[%s] failed to create namespace for %s %s: %w", controller, t.GetKind(), nsName, err)
			}
		}

		return obj, nil
	})
}
