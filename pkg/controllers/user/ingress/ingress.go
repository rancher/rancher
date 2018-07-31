package ingress

import (
	"context"

	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/rancher/norman/types/convert"
	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring ingress and
// creating services for them if the service is missing
// Creation would only happen if the service reference was put by Rancher API based on
// ingress.backend.workloadId. This information is stored in state field in the annotation

type Controller struct {
	serviceLister v1.ServiceLister
	services      v1.ServiceInterface
	clusterName   string
	clusterLister v3.ClusterLister
}

func Register(ctx context.Context, workload *config.UserOnlyContext, cluster *config.UserContext) {
	c := &Controller{
		services:      workload.Core.Services(""),
		serviceLister: workload.Core.Services("").Controller().Lister(),
		clusterName:   workload.ClusterName,
		clusterLister: cluster.Management.Management.Clusters("").Controller().Lister(),
	}
	workload.Extensions.Ingresses("").AddHandler("ingressWorkloadController", c.sync)
}

func (c *Controller) sync(key string, obj *v1beta1.Ingress) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	state := GetIngressState(obj)
	if state == nil {
		return nil
	}

	expectedServices, err := generateExpectedServices(state, obj)
	if err != nil {
		return err
	}

	existingServices, err := getIngressRelatedServices(c.serviceLister, obj, expectedServices)
	if err != nil {
		return err
	}

	needNodePort := c.needNodePort()

	// 1. clean up first, delete or update the service is existing
	for _, service := range existingServices {
		shouldDelete, toUpdate := updateOrDelete(obj, service, expectedServices, needNodePort)
		if shouldDelete {
			if err := c.services.DeleteNamespaced(obj.Namespace, service.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
			continue
		}
		if toUpdate != nil {
			if _, err := c.services.Update(toUpdate); err != nil {
				return err
			}
		}
		// don't create the services which already exist
		delete(expectedServices, service.Name)
	}

	// 2. create the new services
	for _, ingressService := range expectedServices {
		var toCreate *corev1.Service
		if needNodePort {
			toCreate = ingressService.generateNewService(obj, corev1.ServiceTypeNodePort)
		} else {
			toCreate = ingressService.generateNewService(obj, corev1.ServiceTypeClusterIP)
		}
		logrus.Infof("Creating %s service %s for ingress %s, port %d", ingressService.serviceName, toCreate.Spec.Type, key, ingressService.servicePort)
		if _, err := c.services.Create(toCreate); err != nil {
			return err
		}
	}

	return nil
}

func generateExpectedServices(state map[string]string, obj *v1beta1.Ingress) (map[string]ingressService, error) {
	var err error
	rtn := map[string]ingressService{}
	for _, r := range obj.Spec.Rules {
		host := r.Host
		for _, b := range r.HTTP.Paths {
			key := GetStateKey(obj.Name, obj.Namespace, host, b.Path, convert.ToString(b.Backend.ServicePort.IntVal))
			if workloadIDs, ok := state[key]; ok {
				rtn[b.Backend.ServiceName], err = generateIngressService(b.Backend.ServiceName, b.Backend.ServicePort.IntVal, workloadIDs)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if obj.Spec.Backend != nil {
		key := GetStateKey(obj.Name, obj.Namespace, "", "/", convert.ToString(obj.Spec.Backend.ServicePort.IntVal))
		if workloadIDs, ok := state[key]; ok {
			rtn[obj.Spec.Backend.ServiceName], err = generateIngressService(obj.Spec.Backend.ServiceName, obj.Spec.Backend.ServicePort.IntVal, workloadIDs)
			if err != nil {
				return nil, err
			}
		}
	}
	return rtn, nil
}

func getIngressRelatedServices(serviceLister v1.ServiceLister, obj *v1beta1.Ingress, expectedServices map[string]ingressService) (map[string]*corev1.Service, error) {
	rtn := map[string]*corev1.Service{}
	services, err := serviceLister.List(obj.Namespace, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for _, service := range services {
		//mark the service which related to ingress
		if _, ok := expectedServices[service.Name]; ok {
			rtn[service.Name] = service
			continue
		}
		//mark the service which own by ingress but not related to ingress
		if IsServiceOwnedByIngress(obj, service) {
			rtn[service.Name] = service
		}
	}
	return rtn, nil
}

func updateOrDelete(obj *v1beta1.Ingress, service *corev1.Service, expectedServices map[string]ingressService, isNeedNodePort bool) (bool, *corev1.Service) {
	shouldDelete := false
	var toUpdate *corev1.Service
	s, ok := expectedServices[service.Name]
	if ok {
		if service.Annotations == nil {
			service.Annotations = map[string]string{}
		}
		// handling issue https://github.com/rancher/rancher/issues/13717.
		// if node port is using by non-GKE for ingress service, we should replace them.
		if service.Spec.Type == corev1.ServiceTypeNodePort && !isNeedNodePort && IsServiceOwnedByIngress(obj, service) {
			shouldDelete = true
		} else {
			if service.Annotations[util.WorkloadAnnotation] != s.workloadIDs && s.workloadIDs != "" {
				toUpdate = service.DeepCopy()
				toUpdate.Annotations[util.WorkloadAnnotation] = s.workloadIDs
			}
		}
	} else {
		//delete those service owned by ingress
		if IsServiceOwnedByIngress(obj, service) {
			shouldDelete = true
		}
	}
	return shouldDelete, toUpdate
}

func (c *Controller) needNodePort() bool {
	cluster, err := c.clusterLister.Get("", c.clusterName)
	if err != nil || cluster.DeletionTimestamp != nil {
		return false
	}
	if cluster.Spec.GoogleKubernetesEngineConfig != nil {
		return true
	}
	return false
}
