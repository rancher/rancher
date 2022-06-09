package nslabels

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	typescorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ProjectIDFieldLabel           = "field.cattle.io/projectId"
	ProjectScopedSecretAnnotation = "secret.user.cattle.io/secret"
)

type namespaceHandler struct {
	secrets  v1.SecretInterface
	nsClient typescorev1.NamespaceInterface
}

func Register(ctx context.Context, cluster *config.UserOnlyContext) {
	logrus.Infof("Registering namespaceHandler for adding labels ")
	nsh := &namespaceHandler{
		secrets:  cluster.Core.Secrets(""),
		nsClient: cluster.Core.Namespaces(""),
	}
	cluster.Core.Namespaces("").AddHandler(ctx, "namespaceHandler", nsh.Sync)
}

func (nsh *namespaceHandler) Sync(key string, ns *corev1.Namespace) (runtime.Object, error) {
	if ns == nil {
		return nil, nil
	}
	logrus.Debugf("namespaceHandler: Sync: key=%v, ns=%+v", key, *ns)

	field, ok := ns.Annotations[ProjectIDFieldLabel]
	if !ok {
		return nil, nil
	}

	projectID := ""
	clusterID := ""
	if field != "" {
		splits := strings.Split(field, ":")
		if len(splits) != 2 {
			return nil, nil
		}
		projectID = splits[1]
		clusterID = splits[0]
	}

	logrus.Debugf("namespaceHandler: Sync: projectID=%v", projectID)

	if err := nsh.addProjectIDLabelToNamespace(ns, projectID, clusterID); err != nil {
		logrus.Errorf("namespaceHandler: Sync: error adding project id label to namespace err=%v", err)
		return nil, nil
	}

	return nil, nil
}

func (nsh *namespaceHandler) addProjectIDLabelToNamespace(ns *corev1.Namespace, projectID string, clusterID string) error {
	if ns == nil {
		return fmt.Errorf("cannot add label to nil namespace")
	}
	if ns.Labels[ProjectIDFieldLabel] != projectID {
		if err := nsh.updateProjectIDLabelForSecrets(projectID, ns.Name, clusterID); err != nil {
			logrus.Trace(err)
		}
		logrus.Infof("namespaceHandler: addProjectIDLabelToNamespace: adding label %v=%v to namespace=%v", ProjectIDFieldLabel, projectID, ns.Name)
		nscopy := ns.DeepCopy()
		if nscopy.Labels == nil {
			nscopy.Labels = map[string]string{}
		}
		nscopy.Labels[ProjectIDFieldLabel] = projectID
		if _, err := nsh.nsClient.Update(nscopy); err != nil {
			return err
		}
	}

	return nil
}

func (nsh *namespaceHandler) updateProjectIDLabelForSecrets(projectID string, namespace string, clusterID string) error {
	secrets, err := nsh.secrets.List(metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.namespace=%s", namespace)})
	if err != nil {
		return err
	}
	for _, secret := range secrets.Items {
		if secret.Annotations[ProjectScopedSecretAnnotation] == "true" {
			if err := nsh.secrets.DeleteNamespaced(namespace, secret.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		} else {
			secretCopy := secret.DeepCopy()
			if secretCopy.Annotations == nil {
				secretCopy.Annotations = make(map[string]string)
			}
			if projectID == "" {
				secretCopy.Annotations[ProjectIDFieldLabel] = projectID
			} else {
				secretCopy.Annotations[ProjectIDFieldLabel] = fmt.Sprintf("%s:%s", clusterID, projectID)
			}
			if _, err := nsh.secrets.Update(secretCopy); err != nil {
				return err
			}
		}
	}
	return nil
}
