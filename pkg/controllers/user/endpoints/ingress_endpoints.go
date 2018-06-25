package endpoints

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/extensions/v1beta1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type IngressEndpointsController struct {
	ingressInterface  v1beta1.IngressInterface
	ingressLister     v1beta1.IngressLister
	serviceLister     v1.ServiceLister
	serviceController v1.ServiceController
	machinesLister    managementv3.NodeLister
	isRKE             bool
	clusterName       string
}

func (c *IngressEndpointsController) sync(key string, obj *extensionsv1beta1.Ingress) error {
	var ingresses []*extensionsv1beta1.Ingress
	var err error

	if obj == nil {
		ingresses, err = c.ingressLister.List("", labels.NewSelector())
		if err != nil {
			return err
		}
	} else {
		ingresses = []*extensionsv1beta1.Ingress{obj}
	}

	for _, ingress := range ingresses {
		for _, service := range c.getServicesFromIngress(ingress) {
			c.serviceController.Enqueue(service.Namespace, service.Name)
		}
	}

	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}

	if _, err := c.reconcileEndpointsForIngress(obj); err != nil {
		return err
	}
	return nil
}

func (c *IngressEndpointsController) reconcileEndpointsForIngress(obj *extensionsv1beta1.Ingress) (bool, error) {
	allNodesIP, err := getAllNodesPublicEndpointIP(c.machinesLister, c.clusterName)
	if err != nil {
		return false, err
	}
	fromObj := convertIngressToPublicEndpoints(obj, c.isRKE, allNodesIP)
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

func (c *IngressEndpointsController) getServicesFromIngress(obj *extensionsv1beta1.Ingress) (services []*corev1.Service) {
	if obj.Spec.Backend != nil {
		svc, err := c.serviceLister.Get(obj.Namespace, obj.Spec.Backend.ServiceName)
		if err != nil {
			logrus.WithError(err).Warnf("can not get service %s:%s when refresh ingress %s endpoint", obj.Namespace, obj.Spec.Backend.ServiceName, obj.Name)
		} else {
			services = append(services, svc)
		}
	}
	for _, rule := range obj.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			svc, err := c.serviceLister.Get(obj.Namespace, path.Backend.ServiceName)
			if err != nil {
				logrus.WithError(err).Warnf("can not get service %s:%s when refresh ingress %s endpoint", obj.Namespace, path.Backend.ServiceName, obj.Name)
			} else {
				services = append(services, svc)
			}
		}
	}
	return services
}
