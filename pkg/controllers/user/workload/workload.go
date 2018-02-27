package workload

import (
	"context"
	"fmt"
	"reflect"

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
	return c.CreateServiceForWorkload(w)
}

func (c *Controller) serviceExistsForWorkload(workload *Workload, service *Service) (bool, error) {
	labels := fmt.Sprintf("%s=%s", WorkloadLabel, workload.Namespace)
	services, err := c.services.List(metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return false, err
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
				return true, nil
			}
		}
	}
	return false, nil
}

func (c *Controller) CreateServiceForWorkload(workload *Workload) error {
	// do not create if object is "owned" by other workload
	for _, o := range workload.OwnerReferences {
		if ok := WorkloadKinds[o.Kind]; ok {
			return nil
		}
	}

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
	controller := true
	ownerRef := metav1.OwnerReference{
		Name:       workload.Name,
		APIVersion: workload.APIVersion,
		UID:        workload.UUID,
		Kind:       workload.Kind,
		Controller: &controller,
	}
	// we use workload annotation instead of service.selector (based off workload.labels)
	// to avoid service recreate on user label change
	serviceAnnotations := map[string]string{}
	serviceAnnotations[WorkloadAnnotation] = workload.getKey()
	// labels are used so it is easy to filter out the service when query from cache
	serviceLabels := map[string]string{}
	serviceLabels[WorkloadLabel] = workload.Namespace

	for kind, toCreate := range services {
		exists, err := c.serviceExistsForWorkload(workload, &toCreate)
		if err != nil {
			return err
		}
		if exists {
			//TODO - implement update once workload upgrade is supported
			continue
		}
		serviceType := toCreate.Type
		if toCreate.ClusterIP == "None" {
			serviceType = "Headless"
			serviceAnnotations[WorkloadAnnotation] = workload.getKey()
		}
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName:    "workload-",
				OwnerReferences: []metav1.OwnerReference{ownerRef},
				Namespace:       workload.Namespace,
				Annotations:     serviceAnnotations,
				Labels:          serviceLabels,
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: toCreate.ClusterIP,
				Type:      kind,
				Ports:     toCreate.ServicePorts,
			},
		}

		logrus.Infof("Creating [%s] service with ports [%v] for workload %s", serviceType, toCreate.ServicePorts, workload.getKey())
		_, err = c.services.Create(service)
		if err != nil {
			return err
		}
	}

	return nil
}
