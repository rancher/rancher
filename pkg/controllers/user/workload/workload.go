package workload

import (
	"context"
	"encoding/json"
	"reflect"

	"strings"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
)

// This controller is responsible for monitoring workloads and
// creating services for them
// a) when rancher ports annotation is present, create service based on annotation ports
// b) when annotation is missing, create a headless service

const (
	creatorIDAnnotation = "field.cattle.io/creatorId"
)

type Controller struct {
	workloadController CommonController
	serviceLister      v1.ServiceLister
	services           v1.ServiceInterface
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &Controller{
		serviceLister: workload.Core.Services("").Controller().Lister(),
		services:      workload.Core.Services(""),
	}
	c.workloadController = NewWorkloadController(workload, c.CreateService)
}

func getName() string {
	return "workloadServiceGenerationController"
}

func (c *Controller) CreateService(key string, w *Workload) error {
	// do not create service for job, cronJob and for workload owned by controller (ReplicaSet)
	if strings.EqualFold(w.Kind, "job") || strings.EqualFold(w.Kind, "cronJob") {
		return nil
	}
	for _, o := range w.OwnerReferences {
		if *o.Controller {
			return nil
		}
	}

	if _, ok := w.Annotations[creatorIDAnnotation]; !ok {
		return nil
	}

	if errs := validation.IsDNS1123Subdomain(w.Name); len(errs) != 0 {
		logrus.Debugf("Not creating service for workload [%s]: dns name is invalid", w.Name)
		return nil
	}

	return c.CreateServiceForWorkload(w)
}

func (c *Controller) serviceExistsForWorkload(workload *Workload, service *Service) (*corev1.Service, error) {
	s, err := c.serviceLister.Get(workload.Namespace, service.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if s.DeletionTimestamp != nil {
		return nil, nil
	}

	return s, nil
}

func (c *Controller) CreateServiceForWorkload(workload *Workload) error {
	services := map[corev1.ServiceType]Service{}
	if _, ok := workload.TemplateSpec.Annotations[PortsAnnotation]; ok {
		svcs, err := generateServicesFromPortsAnnotation(workload)
		if err != nil {
			return err
		}
		for _, service := range svcs {
			services[service.Type] = service
		}
	} else {
		service := generateServiceFromContainers(workload)
		services[service.Type] = *service
	}

	existingServices, err := c.getExistingServices(workload)
	if err != nil {
		return err
	}

	for _, toCreate := range services {
		existing, ok := existingServices[toCreate.Name]

		if !ok {
			if err := c.createService(toCreate, workload); err != nil {
				return err
			}

		} else {

			if arePortsEqual(toCreate.ServicePorts, existing.Spec.Ports) && existing.Spec.Type == toCreate.Type {
				continue
			}

			if err := c.updateService(toCreate, existing); err != nil {
				return err
			}
		}
	}

	for name, svc := range existingServices {
		found := false
		for _, toCreate := range services {
			if toCreate.Name == name {
				found = true
				break
			}
		}
		if !found {
			if err := c.deleteService(svc); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Controller) updateService(toUpdate Service, existing *corev1.Service) error {

	if existing.Spec.Type == toUpdate.Type {
		existingPortNameToPort := map[string]corev1.ServicePort{}
		for _, p := range existing.Spec.Ports {
			existingPortNameToPort[p.Name] = p
		}

		var portsToUpdate []corev1.ServicePort
		for _, p := range toUpdate.ServicePorts {
			if val, ok := existingPortNameToPort[p.Name]; ok {
				if val.Port == p.Port {
					// Once switch to k8s 1.9, reset only when p.Nodeport == 0. There is a bug in 1.8
					// on port update with diff NodePort value resulting in api server crash
					// https://github.com/kubernetes/kubernetes/issues/58892
					//if p.NodePort == 0 {
					//	p.NodePort = val.NodePort
					//}
					p.NodePort = val.NodePort
				}
			}
			portsToUpdate = append(portsToUpdate, p)
		}

		existing.Spec.Ports = portsToUpdate
	} else {
		existing.Spec.Ports = toUpdate.ServicePorts
		existing.Spec.Type = toUpdate.Type
	}

	logrus.Infof("Updating [%s/%s] service with ports [%v]", existing.Namespace, existing.Name, existing.Spec.Ports)
	_, err := c.services.Update(existing)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) createService(toCreate Service, workload *Workload) error {
	controller := true
	ownerRef := metav1.OwnerReference{
		Name:       workload.Name,
		APIVersion: workload.APIVersion,
		UID:        workload.UUID,
		Kind:       workload.Kind,
		Controller: &controller,
	}

	serviceAnnotations := map[string]string{}
	workloadAnnotationValue, err := workloadAnnotationToString(workload.getKey())
	if err != nil {
		return err
	}
	serviceAnnotations[WorkloadAnnotation] = workloadAnnotationValue
	serviceAnnotations[WorkloadAnnotatioNoop] = "true"

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Namespace:       workload.Namespace,
			Name:            workload.Name,
			Annotations:     serviceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: toCreate.ClusterIP,
			Type:      toCreate.Type,
			Ports:     toCreate.ServicePorts,
			Selector:  workload.SelectorLabels,
		},
	}

	logrus.Infof("Creating [%s] service with ports [%v] for workload %s", service.Spec.Type, toCreate.ServicePorts, workload.getKey())
	_, err = c.services.Create(service)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func arePortsEqual(one []corev1.ServicePort, two []corev1.ServicePort) bool {
	if len(one) != len(two) {
		return false
	}

	for _, o := range one {
		found := false
		for _, t := range two {
			// Once switch to k8s 1.9, compare nodePort value as well. There is a bug in 1.8
			// on port update with diff NodePort value resulting in api server crash
			// https://github.com/kubernetes/kubernetes/issues/58892
			if o.TargetPort == t.TargetPort && o.Protocol == t.Protocol && o.Port == t.Port {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func workloadAnnotationToString(workloadID string) (string, error) {
	ws := []string{workloadID}
	b, err := json.Marshal(ws)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Controller) getExistingServices(workload *Workload) (map[string]*corev1.Service, error) {
	svcs, err := c.serviceLister.List(workload.Namespace, labels.Everything())
	if err != nil {
		return nil, err
	}

	existingSvcs := map[string]*corev1.Service{}

	for _, svc := range svcs {
		if svc.DeletionTimestamp != nil {
			continue
		}

		isOwner := false
		for _, ref := range svc.OwnerReferences {
			if reflect.DeepEqual(ref.UID, workload.UUID) {
				isOwner = true
				break
			}
		}
		if !isOwner {
			continue
		}

		existingSvcs[svc.Name] = svc
	}

	return existingSvcs, nil

}

func (c *Controller) deleteService(toDelete *corev1.Service) error {

	err := c.services.DeleteNamespaced(toDelete.Namespace, toDelete.Name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
