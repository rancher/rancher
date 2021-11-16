package endpoints

import (
	"reflect"
	"strings"

	workloadutil "github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	"github.com/rancher/rancher/pkg/ingresswrapper"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

type IngressEndpointsController struct {
	workloadController workloadutil.CommonController
	ingressInterface   ingresswrapper.CompatInterface
}

func (c *IngressEndpointsController) sync(key string, obj ingresswrapper.Ingress) (runtime.Object, error) {
	namespace := ""
	if obj != nil && !reflect.ValueOf(obj).IsNil() {
		namespace = obj.GetNamespace()
	} else {
		split := strings.Split(key, "/")
		if len(split) == 2 {
			namespace = split[0]
		}
	}
	c.workloadController.EnqueueAllWorkloads(namespace)

	if obj == nil || reflect.ValueOf(obj).IsNil() || obj.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	if _, err := c.reconcileEndpointsForIngress(obj); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *IngressEndpointsController) reconcileEndpointsForIngress(obj ingresswrapper.Ingress) (bool, error) {
	fromObj, err := convertIngressToPublicEndpoints(obj, settings.IsRKE.Get() == "true")
	if err != nil {
		return false, err
	}
	fromAnnotation := getPublicEndpointsFromAnnotations(obj.GetAnnotations())

	if areEqualEndpoints(fromAnnotation, fromObj) {
		return false, nil
	}

	epsToUpdate, err := publicEndpointsToString(fromObj)
	if err != nil {
		return false, err
	}

	logrus.Infof("Updating ingress [%s:%s] with public endpoints [%v]", obj.GetNamespace(), obj.GetName(), epsToUpdate)

	toUpdate, err := ingresswrapper.ToCompatIngress(obj.DeepCopyObject())
	if err != nil {
		return false, err
	}

	annotations := toUpdate.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[endpointsAnnotation] = epsToUpdate
	toUpdate.SetAnnotations(annotations)

	_, err = c.ingressInterface.Update(toUpdate)
	return false, err
}
