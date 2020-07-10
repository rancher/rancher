package istio

import (
	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/project"
	mgmtv3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type clusterHandler struct {
	clusterName      string
	appLister        v3.AppLister
	clusterInterface mgmtv3.ClusterInterface
	projectLister    mgmtv3.ProjectLister
}

func (h *clusterHandler) sync(key string, obj *mgmtv3.Cluster) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Name != h.clusterName {
		return obj, nil
	}
	if !mgmtv3.ClusterConditionconditionSystemProjectCreated.IsTrue(obj) {
		return obj, nil
	}
	enabled, err := h.getIstioAppEnabled()
	if err != nil {
		return obj, &controller.ForgetError{Err: err, Reason: "Failed to find istio app"}
	}
	if obj.Status.IstioEnabled == enabled {
		return obj, nil
	}
	newCluster := obj.DeepCopy()
	newCluster.Status.IstioEnabled = enabled

	return h.clusterInterface.Update(newCluster)
}

func (h *clusterHandler) getIstioAppEnabled() (bool, error) {
	p, err := project.GetSystemProject(h.clusterName, h.projectLister)
	if err != nil {
		return false, err
	}

	app, err := h.appLister.Get(p.Name, istioAppName)
	if errors.IsNotFound(err) || app.DeletionTimestamp != nil {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}
