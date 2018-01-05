package secrets

import (
	"strings"

	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	projectIDLabel = "field.cattle.io/projectId"
)

func Register(management *config.ManagementContext) {
	lifecycle := &Lifecycle{
		K8sClient:  management.K8sClient,
		Management: management,
	}
	management.Core.Secrets("").AddLifecycle("secrets", lifecycle)
}

type Lifecycle struct {
	K8sClient  kubernetes.Interface
	Management *config.ManagementContext
}

func (l *Lifecycle) Create(obj *v1.Secret) (*v1.Secret, error) {
	return l.createOrUpdate(obj, "create")
}

func (l *Lifecycle) Remove(obj *v1.Secret) (*v1.Secret, error) {
	projectID := obj.Namespace
	if projectID != "" && strings.HasPrefix(projectID, "project-") {
		namespaces, err := l.Management.Core.Namespaces("").Controller().Lister().List("", labels.Everything())
		if err != nil {
			return obj, err
		}
		for _, namespace := range namespaces {
			parts := strings.Split(namespace.Annotations[projectIDLabel], ":")
			if len(parts) == 2 && parts[1] == projectID {
				logrus.Infof("deleting secrets %s in namespace %s", obj.Name, namespace.Name)
				if err := l.K8sClient.CoreV1().Secrets(namespace.Name).Delete(obj.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					return obj, err
				}
			}
		}
	}
	return obj, nil
}

func (l *Lifecycle) Updated(obj *v1.Secret) (*v1.Secret, error) {
	return l.createOrUpdate(obj, "update")
}

func (l *Lifecycle) createOrUpdate(obj *v1.Secret, action string) (*v1.Secret, error) {
	projectID := obj.Namespace
	if projectID == "" || !strings.HasPrefix(projectID, "project-") {
		return obj, nil
	}
	namespaces, err := l.Management.Core.Namespaces("").Controller().Lister().List("", labels.Everything())
	if err != nil {
		return obj, err
	}
	for _, namespace := range namespaces {
		parts := strings.Split(namespace.Annotations[projectIDLabel], ":")
		if len(parts) == 2 && parts[1] == projectID {
			// copy the secret into namespace
			namespacedSecret := &v1.Secret{}
			namespacedSecret.Name = obj.Name
			namespacedSecret.Annotations = obj.Annotations
			namespacedSecret.Kind = obj.Kind
			namespacedSecret.Data = obj.Data
			namespacedSecret.StringData = obj.StringData
			namespacedSecret.Type = obj.Type
			switch action {
			case "create":
				logrus.Infof("Copying secrets %s into namespace %s", namespacedSecret.Name, namespace.Name)
				_, err := l.K8sClient.CoreV1().Secrets(namespace.Name).Create(namespacedSecret)
				if err != nil && !errors.IsAlreadyExists(err) {
					return obj, err
				}
			case "update":
				_, err := l.K8sClient.CoreV1().Secrets(namespace.Name).Update(namespacedSecret)
				if err != nil && !errors.IsNotFound(err) {
					return obj, err
				} else if errors.IsNotFound(err) {
					_, err := l.K8sClient.CoreV1().Secrets(namespace.Name).Create(namespacedSecret)
					if err != nil {
						return obj, err
					}
				}
			}
		}
	}

	return obj, nil
}
