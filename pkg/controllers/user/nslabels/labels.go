package nslabels

import (
	"fmt"
	"strings"

	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	ProjectIDFieldLabel = "field.cattle.io/projectId"
)

type namespaceHandler struct {
	nsClient typescorev1.NamespaceInterface
}

func Register(cluster *config.UserContext) {
	logrus.Infof("Registering namespaceHandler for adding labels ")
	nsh := &namespaceHandler{
		cluster.Core.Namespaces(""),
	}
	cluster.Core.Namespaces("").AddHandler("namespaceHandler", nsh.Sync)
}

func (nsh *namespaceHandler) Sync(key string, ns *corev1.Namespace) error {
	if ns == nil {
		return nil
	}
	logrus.Debugf("namespaceHandler: Sync: key=%v, ns=%+v", key, *ns)

	field, ok := ns.Annotations[ProjectIDFieldLabel]
	if !ok {
		return nil
	}

	splits := strings.Split(field, ":")
	if len(splits) != 2 {
		return nil
	}
	projectID := splits[1]
	logrus.Debugf("namespaceHandler: Sync: projectID=%v", projectID)

	if err := nsh.addProjectIDLabelToNamespace(ns, projectID); err != nil {
		logrus.Errorf("namespaceHandler: Sync: error adding project id label to namespace err=%v", err)
		return nil
	}

	return nil
}

func (nsh *namespaceHandler) addProjectIDLabelToNamespace(ns *corev1.Namespace, projectID string) error {
	if ns == nil {
		return fmt.Errorf("cannot add label to nil namespace")
	}
	if ns.Labels[ProjectIDFieldLabel] != projectID {
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
