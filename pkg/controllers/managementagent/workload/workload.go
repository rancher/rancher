package workload

import (
	"context"
	"encoding/json"
	"reflect"

	"strings"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
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
	c.workloadController = NewWorkloadController(ctx, workload, c.CreateService)
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
		if o.Controller != nil && *o.Controller {
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
	}
	// always create cluster ip service, if missing in ports
	if _, ok := services[ClusterIPServiceType]; !ok {
		service := generateClusterIPServiceFromContainers(workload)
		services[service.Type] = *service
	}

	// 1. Create new services
	for _, toCreate := range services {
		existing, err := c.serviceExistsForWorkload(workload, &toCreate)
		if err != nil {
			return err
		}

		recreate := false
		// to handle clusterIP to headless service updates
		// as clusterIP field is immutable
		if existing != nil && toCreate.Type == ClusterIPServiceType {
			clusterIPNew := toCreate.ClusterIP
			custerIPOld := existing.Spec.ClusterIP
			if clusterIPNew != custerIPOld && (clusterIPNew == "None" || custerIPOld == "None") {
				err = c.services.DeleteNamespaced(existing.Namespace, existing.Name, &metav1.DeleteOptions{})
				if err != nil {
					return err
				}
				recreate = true
			}
		}

		if existing == nil || recreate {
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

			if ArePortsEqual(toCreate.ServicePorts, existing.Spec.Ports) {
				continue
			}

			if err := c.updateService(toCreate, existing); err != nil {
				return err
			}
		}
	}
	// 2. Cleanup services that are no longer needed
	existingSvcs, err := c.getServicesOwnedByWorkload(workload)
	if err != nil {
		return err
	}
	var toRemove []*corev1.Service
	for _, existingSvc := range existingSvcs {
		toCreate, ok := services[existingSvc.Spec.Type]
		if ok && toCreate.Name == existingSvc.Name {
			continue
		}
		toRemove = append(toRemove, existingSvc)
	}
	for _, svc := range toRemove {
		logrus.Infof("Deleting [%s/%s] service of type [%s] for workload [%s/%s]", svc.Namespace, svc.Name, svc.Spec.Type,
			workload.Namespace, workload.Name)
		if err := c.services.DeleteNamespaced(svc.Namespace, svc.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) updateService(toUpdate Service, existing *corev1.Service) error {
	existingPortNameToPort := map[string]corev1.ServicePort{}
	for _, p := range existing.Spec.Ports {
		existingPortNameToPort[p.Name] = p
	}

	var portsToUpdate []corev1.ServicePort
	for _, p := range toUpdate.ServicePorts {
		if val, ok := existingPortNameToPort[p.Name]; ok {
			if val.Port == p.Port {
				if p.NodePort != val.NodePort {
					if p.NodePort == 0 {
						// random port handling to avoid infinite updates
						p.NodePort = val.NodePort
					}
				}
			}
		}
		portsToUpdate = append(portsToUpdate, p)
	}

	existing.Spec.Ports = portsToUpdate
	if existing.Spec.Type == ClusterIPServiceType && existing.Spec.ClusterIP == "None" {
		existing.Spec.ClusterIP = toUpdate.ClusterIP
	}
	logrus.Infof("Updating [%s/%s] service with ports [%v]", existing.Namespace, existing.Name, portsToUpdate)
	_, err := c.services.Update(existing)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) getServicesOwnedByWorkload(workload *Workload) ([]*corev1.Service, error) {
	var toReturn []*corev1.Service
	services, err := c.serviceLister.List(workload.Namespace, labels.NewSelector())
	if err != nil {
		return toReturn, err
	}
	for _, svc := range services {
		if _, ok := svc.Annotations[WorkloaAnnotationdPortBasedService]; ok {
			for _, o := range svc.OwnerReferences {
				if o.UID == workload.UUID {
					toReturn = append(toReturn, svc)
					break
				}
			}
		}
	}
	return toReturn, nil
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
	workloadAnnotationValue, err := IDAnnotationToString(workload.Key)
	if err != nil {
		return err
	}
	serviceAnnotations[WorkloadAnnotation] = workloadAnnotationValue
	serviceAnnotations[WorkloadAnnotatioNoop] = "true"
	serviceAnnotations[WorkloaAnnotationdPortBasedService] = "true"

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Namespace:       workload.Namespace,
			Name:            toCreate.Name,
			Annotations:     serviceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: toCreate.ClusterIP,
			Type:      toCreate.Type,
			Ports:     toCreate.ServicePorts,
			Selector:  workload.SelectorLabels,
		},
	}

	logrus.Infof("Creating [%s/%s] service of type [%s] with ports [%v] for workload %s", service.Namespace, service.Name,
		service.Spec.Type, toCreate.ServicePorts, workload.Key)
	_, err = c.services.Create(service)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func ArePortsEqual(one []corev1.ServicePort, two []corev1.ServicePort) bool {
	if len(one) != len(two) {
		return false
	}

	for _, o := range one {
		found := false
		for _, t := range two {
			nodePortsEqual := (o.NodePort == 0 || t.NodePort == 0) || (o.NodePort == t.NodePort)
			if o.TargetPort == t.TargetPort && o.Protocol == t.Protocol && o.Port == t.Port && nodePortsEqual {
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

func IDAnnotationToString(workloadID string) (string, error) {
	ws := []string{workloadID}
	b, err := json.Marshal(ws)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
