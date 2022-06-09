package ingress

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/norman/types/convert"
	util "github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/ingresswrapper"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	nodeLister    v1.NodeLister
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &Controller{
		services:      workload.Core.Services(""),
		serviceLister: workload.Core.Services("").Controller().Lister(),
		nodeLister:    workload.Core.Nodes("").Controller().Lister(),
	}
	if ingresswrapper.ServerSupportsIngressV1(workload.K8sClient) {
		workload.Networking.Ingresses("").AddHandler(ctx, "ingressWorkloadController", ingresswrapper.CompatSyncV1(c.sync))
	} else {
		workload.Extensions.Ingresses("").AddHandler(ctx, "ingressWorkloadController", ingresswrapper.CompatSyncV1Beta1(c.sync))
	}
}

func (c *Controller) sync(key string, ingress ingresswrapper.Ingress) (runtime.Object, error) {
	if ingress == nil || reflect.ValueOf(ingress).IsNil() || ingress.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	obj, err := ingresswrapper.ToCompatIngress(ingress)
	if err != nil {
		return obj, err
	}
	state := GetIngressState(obj)
	if state == nil {
		return nil, nil
	}

	expectedServices, err := generateExpectedServices(state, obj)
	if err != nil {
		return nil, err
	}

	existingServices, err := getIngressRelatedServices(c.serviceLister, obj, expectedServices)
	if err != nil {
		return nil, err
	}

	needNodePort := c.needNodePort()

	// 1. clean up first, delete or update the service is existing
	for _, service := range existingServices {
		shouldDelete, toUpdate, err := updateOrDelete(obj, service, expectedServices, needNodePort)
		if err != nil {
			return nil, err
		}
		if shouldDelete {
			if err := c.services.DeleteNamespaced(obj.GetNamespace(), service.Name, &metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
			continue
		}
		if toUpdate != nil {
			if _, err := c.services.Update(toUpdate); err != nil {
				return nil, err
			}
		}
		// don't create the services which already exist
		delete(expectedServices, service.Name)
	}

	// 2. create the new services
	for _, ingressService := range expectedServices {
		var toCreate *corev1.Service
		var err error
		if needNodePort {
			toCreate, err = ingressService.generateNewService(obj, corev1.ServiceTypeNodePort)
		} else {
			toCreate, err = ingressService.generateNewService(obj, corev1.ServiceTypeClusterIP)
		}
		if err != nil {
			return nil, err
		}
		logrus.Infof("Creating %s service %s for ingress %s, port %d", ingressService.serviceName, toCreate.Spec.Type, key, ingressService.servicePort)
		if _, err := c.services.Create(toCreate); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func generateExpectedServices(state map[string]string, obj *ingresswrapper.CompatIngress) (map[string]ingressService, error) {
	var err error
	rtn := map[string]ingressService{}
	for _, r := range obj.Spec.Rules {
		host := r.Host
		if r.HTTP == nil {
			continue
		}
		for _, b := range r.HTTP.Paths {
			key := GetStateKey(obj.GetName(), obj.GetNamespace(), host, b.Path, convert.ToString(b.Backend.Service.Port.Number))
			if workloadIDs, ok := state[key]; ok {
				rtn[b.Backend.Service.Name], err = generateIngressService(b.Backend.Service.Name, b.Backend.Service.Port.Number, workloadIDs)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if obj.Spec.DefaultBackend != nil {
		key := GetStateKey(obj.GetName(), obj.GetNamespace(), "", "/", convert.ToString(obj.Spec.DefaultBackend.Service.Port.Number))
		if workloadIDs, ok := state[key]; ok {
			rtn[obj.Spec.DefaultBackend.Service.Name], err = generateIngressService(obj.Spec.DefaultBackend.Service.Name, obj.Spec.DefaultBackend.Service.Port.Number, workloadIDs)
			if err != nil {
				return nil, err
			}
		}
	}
	return rtn, nil
}

func getIngressRelatedServices(serviceLister v1.ServiceLister, obj *ingresswrapper.CompatIngress, expectedServices map[string]ingressService) (map[string]*corev1.Service, error) {
	rtn := map[string]*corev1.Service{}
	services, err := serviceLister.List(obj.GetNamespace(), labels.NewSelector())
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
		ok, err := IsServiceOwnedByIngress(obj, service)
		if err != nil {
			return nil, err
		}
		if ok {
			rtn[service.Name] = service
		}
	}
	return rtn, nil
}

func updateOrDelete(obj *ingresswrapper.CompatIngress, service *corev1.Service, expectedServices map[string]ingressService, isNeedNodePort bool) (bool, *corev1.Service, error) {
	shouldDelete := false
	var toUpdate *corev1.Service
	serviceIsOwnedByIngress, err := IsServiceOwnedByIngress(obj, service)
	if err != nil {
		return false, nil, err
	}
	s, ok := expectedServices[service.Name]
	if ok {
		if service.Annotations == nil {
			service.Annotations = map[string]string{}
		}
		// handling issue https://github.com/rancher/rancher/issues/13717.
		// if node port is using by non-GKE for ingress service, we should replace them.
		if service.Spec.Type == corev1.ServiceTypeNodePort && !isNeedNodePort && serviceIsOwnedByIngress {
			shouldDelete = true
		} else {
			if service.Annotations[util.WorkloadAnnotation] != s.workloadIDs && s.workloadIDs != "" {
				toUpdate = service.DeepCopy()
				toUpdate.Annotations[util.WorkloadAnnotation] = s.workloadIDs
			}
		}
	} else {
		//delete those service owned by ingress
		if serviceIsOwnedByIngress {
			shouldDelete = true
		}
	}
	return shouldDelete, toUpdate, nil
}

func (c *Controller) needNodePort() bool {
	nodes, err := c.nodeLister.List("", labels.NewSelector())
	if err != nil {
		return false
	}

	for _, node := range nodes {
		if label, ok := node.Labels["cloud.google.com/gke-nodepool"]; ok && label != "" {
			return true
		}
	}
	return false
}
