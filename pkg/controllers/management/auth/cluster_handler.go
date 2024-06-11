package auth

import (
	"errors"
	"reflect"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	clusterCreateController = "mgmt-cluster-rbac-delete" // TODO the word delete here is wrong, but changing it would break backwards compatibility
	clusterRemoveController = "mgmt-cluster-rbac-remove"
)

type clusterLifecycle struct {
	mgr *mgr
}

func (l *clusterLifecycle) sync(key string, orig *apisv3.Cluster) (runtime.Object, error) {
	if orig == nil || !orig.DeletionTimestamp.IsZero() {
		return orig, nil
	}

	obj := orig.DeepCopyObject()
	obj, err := l.mgr.reconcileResourceToNamespace(obj, clusterCreateController)
	if err != nil {
		return nil, err
	}

	obj, err = l.mgr.createDefaultProject(obj)
	if err != nil {
		return nil, err
	}

	obj, err = l.mgr.createSystemProject(obj)
	if err != nil {
		return nil, err
	}
	obj, err = l.mgr.addRTAnnotation(obj, "cluster")
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating cluster %v", clusterCreateController, orig.Name)
		_, err = l.mgr.mgmt.Management.Clusters("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}

	obj, err = l.mgr.reconcileCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating cluster %v", clusterCreateController, orig.Name)
		_, err = l.mgr.mgmt.Management.Clusters("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (l *clusterLifecycle) Create(obj *apisv3.Cluster) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *clusterLifecycle) Updated(obj *apisv3.Cluster) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *clusterLifecycle) Remove(obj *apisv3.Cluster) (runtime.Object, error) {
	if len(obj.Finalizers) > 1 {
		logrus.Debugf("Skipping rbac cleanup for cluster [%s] until all other finalizers are removed.", obj.Name)
		return obj, generic.ErrSkip
	}

	var returnErr error
	set := labels.Set{rbac.RestrictedAdminClusterRoleBinding: "true"}
	rbs, err := l.mgr.rbLister.List(obj.Name, labels.SelectorFromSet(set))
	returnErr = errors.Join(returnErr, err)

	for _, rb := range rbs {
		err := l.mgr.roleBindings.DeleteNamespaced(obj.Name, rb.Name, &v1.DeleteOptions{})
		returnErr = errors.Join(returnErr, err)
	}
	returnErr = errors.Join(
		l.mgr.deleteSystemProject(obj, clusterRemoveController),
		l.mgr.deleteNamespace(obj, clusterRemoveController),
	)
	return obj, returnErr
}
