package workload

import (
	"context"
	"fmt"
	"reflect"

	"strings"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This controller is responsible for monitoring workloads and
// creating services for them
// a) when rancher ports annotation is present, create service based on annotation ports
// b) when annotation is missing, create a headless service

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
	return c.CreateServiceForWorkload(w)
}

func (c *Controller) serviceExistsForWorkload(workload *Workload, service *Service) (*corev1.Service, error) {
	labels := fmt.Sprintf("%s=%s", WorkloadLabel, workload.Namespace)
	services, err := c.services.List(metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, err
	}

	for _, s := range services.Items {
		if s.DeletionTimestamp != nil {
			continue
		}
		if s.Spec.Type != service.Type {
			continue
		}
		for _, ref := range s.OwnerReferences {
			if reflect.DeepEqual(ref.UID, workload.UUID) {
				return &s, nil
			}
		}
	}
	return nil, nil
}

func (c *Controller) CreateServiceForWorkload(workload *Workload) error {
	services := map[corev1.ServiceType]Service{}
	if val, ok := workload.TemplateSpec.Annotations[PortsAnnotation]; ok {
		svcs, err := generateServicesFromPortsAnnotation(val)
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

	for _, toCreate := range services {
		existing, err := c.serviceExistsForWorkload(workload, &toCreate)
		if err != nil {
			return err
		}
		if existing == nil {
			if err := c.createService(toCreate, workload); err != nil {
				return err
			}

		} else {
			if arePortsEqual(toCreate.ServicePorts, existing.Spec.Ports) {
				continue
			}
			if err := c.updateService(toCreate, existing); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Controller) updateService(toUpdate Service, existing *corev1.Service) error {
	existing.Spec.Ports = toUpdate.ServicePorts
	logrus.Infof("Updating [%s/%s] service with ports [%v]", existing.Namespace, existing.Name, toUpdate.ServicePorts)
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

	// labels are used so it is easy to filter out the service when query from API
	serviceLabels := map[string]string{}
	serviceLabels[WorkloadLabel] = workload.Namespace

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Namespace:       workload.Namespace,
			Labels:          serviceLabels,
			Name:            workload.Name,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: toCreate.ClusterIP,
			Type:      toCreate.Type,
			Ports:     toCreate.ServicePorts,
			Selector:  workload.SelectorLabels,
		},
	}

	logrus.Infof("Creating [%s] service with ports [%v] for workload %s", service.Spec.Type, toCreate.ServicePorts, workload.getKey())
	_, err := c.services.Create(service)
	if err != nil {
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
