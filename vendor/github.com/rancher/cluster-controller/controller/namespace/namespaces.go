package namespace

import (
	"strings"

	"github.com/rancher/types/config"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	projectIDLabel = "field.cattle.io/projectId"
)

func Register(management *config.ManagementContext) {
	lifecycle := &Lifecycle{
		K8sClient: management.K8sClient,
	}
	management.Core.Namespaces("").AddLifecycle("namespace-secrets", lifecycle)
}

type Lifecycle struct {
	K8sClient kubernetes.Interface
}

func (l *Lifecycle) Create(obj *v1.Namespace) (*v1.Namespace, error) {
	// if a namespace is created within a project, it will automatically inherit the secret copy inside that project
	if obj.Annotations[projectIDLabel] != "" {
		parts := strings.Split(obj.Annotations[projectIDLabel], ":")
		if len(parts) == 2 {
			secrets, err := l.K8sClient.CoreV1().Secrets(parts[1]).List(metav1.ListOptions{})
			if err != nil {
				return obj, err
			}
			for _, secret := range secrets.Items {
				namespacedSecret := &v1.Secret{}
				namespacedSecret.Name = secret.Name
				namespacedSecret.Annotations = secret.Annotations
				namespacedSecret.Data = secret.Data
				namespacedSecret.StringData = secret.StringData
				namespacedSecret.Type = secret.Type
				_, err := l.K8sClient.CoreV1().Secrets(obj.Name).Create(namespacedSecret)
				if err != nil && !errors.IsAlreadyExists(err) {
					return obj, err
				}
			}
		}
	}
	return obj, nil
}

func (l *Lifecycle) Updated(obj *v1.Namespace) (*v1.Namespace, error) {
	return obj, nil
}

func (l *Lifecycle) Remove(obj *v1.Namespace) (*v1.Namespace, error) {
	return obj, nil
}
