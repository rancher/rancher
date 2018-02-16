package nslabels

import (
	"reflect"
	"strings"

	"github.com/rancher/norman/clientbase"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	projectIDField = "field.cattle.io/projectId"
)

type mgr struct {
	nsLister typescorev1.NamespaceLister
	nsClient *clientbase.ObjectClient
}

type namespaceLifecycle struct {
	mgr *mgr
}

func Register(cluster *config.UserContext) {
	logrus.Infof("Registering namespace labels syncer")
	nslc := &namespaceLifecycle{
		&mgr{
			cluster.Management.Core.Namespaces("").Controller().Lister(),
			cluster.Core.Namespaces("").ObjectClient(),
		}}
	cluster.Core.Namespaces("").AddLifecycle("namespaceLifecycle", nslc)
}

func (nslc *namespaceLifecycle) Create(ns *corev1.Namespace) (*corev1.Namespace, error) {
	return ns, nil
}

func (nslc *namespaceLifecycle) Updated(ns *corev1.Namespace) (*corev1.Namespace, error) {
	logrus.Infof("labels nslc Updated: %+v", *ns)
	metaAccessor, err := meta.Accessor(ns)
	if err != nil {
		return ns, nil
	}

	field, ok := metaAccessor.GetAnnotations()[projectIDField]
	if !ok {
		return ns, nil
	}

	splits := strings.Split(field, ":")
	if len(splits) != 2 {
		return ns, nil
	}
	projectID := splits[1]
	logrus.Debugf("labels nslc Updated: projectID=%v", projectID)

	err = addProjectIDLabelToNamespace(nslc.mgr, ns, projectID)
	if err != nil {
		logrus.Errorf("nslc Updated: error adding project id label to namespace err=%v", err)
		return ns, err
	}

	return ns, nil
}

func (nslc *namespaceLifecycle) Remove(ns *corev1.Namespace) (*corev1.Namespace, error) {
	return ns, nil
}

func addProjectIDLabelToNamespace(mgr *mgr, ns *corev1.Namespace, projectID string) error {
	var err error
	nscopy := ns.DeepCopy()
	if nscopy.Labels == nil {
		nscopy.Labels = map[string]string{}
	}
	nscopy.Labels[projectIDField] = projectID
	if !reflect.DeepEqual(ns, nscopy) {
		logrus.Infof("adding projectId=%v to namespace=%+v", projectID, *ns)
		_, err = mgr.nsClient.Update(nscopy.Name, nscopy)
		if err != nil {
			return err
		}
	}

	return nil
}
