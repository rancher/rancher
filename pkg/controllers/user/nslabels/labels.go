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
	projectIDField = "field.cattle.io/projectId"
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
	logrus.Debugf("namespaceHandler Sync key=%v, ns=%+v", key, *ns)

	field, ok := ns.Annotations[projectIDField]
	if !ok {
		return nil
	}

	splits := strings.Split(field, ":")
	if len(splits) != 2 {
		return nil
	}
	projectID := splits[1]
	logrus.Debugf("namespaceHandler Sync: projectID=%v", projectID)

	if err := nsh.addProjectIDLabelToNamespace(ns, projectID); err != nil {
		logrus.Errorf("nsh Updated: error adding project id label to namespace err=%v", err)
		return nil
	}

	return nil
}

func (nsh *namespaceHandler) addProjectIDLabelToNamespace(ns *corev1.Namespace, projectID string) error {
	if ns == nil {
		return fmt.Errorf("cannot add label to nil namespace")
	}
	if ns.Labels[projectIDField] != projectID {
		logrus.Infof("adding label %v=%v to namespace=%v", projectIDField, projectID, ns.Name)
		nscopy := ns.DeepCopy()
		if nscopy.Labels == nil {
			nscopy.Labels = map[string]string{}
		}
		nscopy.Labels[projectIDField] = projectID
		if _, err := nsh.nsClient.Update(nscopy); err != nil {
			return err
		}
	}

	return nil
}
