package endpoints

import (
	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/extensions/v1beta1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

type IngressEndpointsController struct {
	workloadController workloadutil.CommonController
	ingressInterface   v1beta1.IngressInterface
	machinesLister     managementv3.NodeLister
	isRKE              bool
	clusterName        string
}

func (c *IngressEndpointsController) sync(key string, obj *extensionsv1beta1.Ingress) error {
	namespace := ""
	if obj != nil {
		namespace = obj.Namespace
	}
	c.workloadController.EnqueueAllWorkloads(namespace)

	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}

	if _, err := c.reconcileEndpointsForIngress(obj); err != nil {
		return err
	}
	return nil
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
