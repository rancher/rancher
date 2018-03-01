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
			// check if the port of the same type
			if existing.Spec.Type != toCreate.Type {
				logrus.Warnf("Service [%s/%s] already exists but with diff type. Expected type [%s], actual type [%v]", existing.Name, existing.Namespace, toCreate.Type, existing.Spec.Type)
				return nil
			}
			isOwner := false
			for _, ref := range existing.OwnerReferences {
				if reflect.DeepEqual(ref.UID, workload.UUID) {
					isOwner = true
					break
				}
			}
			if !isOwner {
				logrus.Warnf("Service [%s/%s] already exists but with diff owner", existing.Name, existing.Namespace)
				return nil
			}

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

func workloadAnnotationToString(workloadID string) (string, error) {
	ws := []string{workloadID}
	b, err := json.Marshal(ws)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
