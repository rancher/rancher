package endpoints

import (
	"strings"

	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/types/apis/extensions/v1beta1"
	"github.com/sirupsen/logrus"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type IngressEndpointsController struct {
	workloadController workloadutil.CommonController
	ingressInterface   v1beta1.IngressInterface
	isRKE              bool
}

func (c *IngressEndpointsController) sync(key string, obj *extensionsv1beta1.Ingress) (runtime.Object, error) {
	namespace := ""
	if obj != nil {
		namespace = obj.Namespace
	} else {
		split := strings.Split(key, "/")
		if len(split) == 2 {
			namespace = split[0]
		}
	}
	c.workloadController.EnqueueAllWorkloads(namespace)

	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if _, err := c.reconcileEndpointsForIngress(obj); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *IngressEndpointsController) reconcileEndpointsForIngress(obj *extensionsv1beta1.Ingress) (bool, error) {
	fromObj := convertIngressToPublicEndpoints(obj, c.isRKE)
	fromAnnotation := getPublicEndpointsFromAnnotations(obj.Annotations)

	if areEqualEndpoints(fromAnnotation, fromObj) {
		return false, nil
	}

	epsToUpdate, err := publicEndpointsToString(fromObj)
	if err != nil {
		return false, err
	}

	logrus.Infof("Updating ingress [%s:%s] with public endpoints [%v]", obj.Namespace, obj.Name, epsToUpdate)

	toUpdate := obj.DeepCopy()
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = make(map[string]string)
	}
	toUpdate.Annotations[endpointsAnnotation] = epsToUpdate
	_, err = c.ingressInterface.Update(toUpdate)

	return false, err
}
