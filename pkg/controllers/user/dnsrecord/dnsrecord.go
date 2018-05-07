package dnsrecord

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"sync"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DNSAnnotation       = "field.cattle.io/targetDnsRecordIds"
	SvcTypeExternalName = "ExternalName"
)

var dnsServiceUUIDToTargetEndpointUUIDs sync.Map
var serviceUUIDToHostNameAlias sync.Map

// Controller is responsible for monitoring DNSRecord services
// and populating the endpoint based on target service endpoints.
// The controller DOES NOT monitor the changes to the target endpoints;
// that would be handled in the by EndpointController
type Controller struct {
	endpoints         v1.EndpointsInterface
	services          v1.ServiceInterface
	serviceController v1.ServiceController
	endpointLister    v1.EndpointsLister
	namespaceLister   v1.NamespaceLister
	serviceLister     v1.ServiceLister
}

// EndpointController is responsible for monitoring endpoints
// finding out if they are the part of DNSRecord service
// and calling the update on the target service
type EndpointController struct {
	serviceController v1.ServiceController
	serviceLister     v1.ServiceLister
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &Controller{
		endpoints:         workload.Core.Endpoints(""),
		services:          workload.Core.Services(""),
		serviceController: workload.Core.Services("").Controller(),
		endpointLister:    workload.Core.Endpoints("").Controller().Lister(),
		namespaceLister:   workload.Core.Namespaces("").Controller().Lister(),
		serviceLister:     workload.Core.Services("").Controller().Lister(),
	}

	e := &EndpointController{
		serviceController: workload.Core.Services("").Controller(),
		serviceLister:     workload.Core.Services("").Controller().Lister(),
	}
	workload.Core.Services("").AddHandler("dnsRecordController", c.sync)
	workload.Core.Endpoints("").AddHandler("dnsRecordEndpointsController", e.reconcileServicesForEndpoint)

}

func (c *Controller) sync(key string, obj *corev1.Service) error {
	// no need to handle the remove
	if obj == nil || obj.DeletionTimestamp != nil {
		c.queueUpdateForExtNameSvc(key, true)
		dnsServiceUUIDToTargetEndpointUUIDs.Delete(key)
		return nil
	}
	c.queueUpdateForExtNameSvc(key, false)
	return c.reconcileEndpoints(key, obj)
}

func (c *Controller) reconcileEndpoints(key string, obj *corev1.Service) error {
	// only process services having targetDNSRecordIds in annotation
	if obj.Annotations == nil {
		return nil
	}
	value, ok := obj.Annotations[DNSAnnotation]
	if !ok || value == "" {
		return nil
	}

	var records []string
	err := json.Unmarshal([]byte(value), &records)
	if err != nil {
		// just log the error, can't really do anything here.
		logrus.Debugf("Failed to unmarshal targetDnsRecordIds", err)
		return nil
	}

	var newEndpointSubsets []corev1.EndpointSubset
	targetEndpointUUIDs := make(map[string]bool)
	toHandleExternalName := false
	var externalName string
	for _, record := range records {
		groomed := strings.TrimSpace(record)
		namespaceService := strings.Split(groomed, ":")
		if len(namespaceService) < 2 {
			return fmt.Errorf("wrong format for dns record [%s]", groomed)
		}
		namespace := namespaceService[0]
		service := namespaceService[1]
		targetEndpoint, err := c.endpointLister.Get(namespace, service)
		if err != nil {
			exists, name, beingDeleted, err := c.checkLinkedSvc(namespace, service, obj.Spec.ExternalName)
			if err != nil {
				return errors.Wrapf(err, "Error fetching dns hostName service [%s] in namespace [%s] : %s", service, namespace, err)
			}
			aliasExtType := obj.Spec.Type == SvcTypeExternalName
			if aliasExtType && beingDeleted {
				logrus.Debugf("Cannot fetch dns hostName service [%s] in namespace [%s] : it is being deleted", service, namespace)
			}
			if exists || aliasExtType {
				toHandleExternalName = true
				externalName = name
				svcKey := fmt.Sprintf("%s/%s", namespace, service)
				serviceUUIDToHostNameAlias.Store(svcKey, key)
				continue
			}
			logrus.Warnf("Failed to fetch endpoints for dns record [%s]: [%v]", groomed, err)
			continue
		}
		if targetEndpoint.DeletionTimestamp != nil {
			logrus.Warnf("Failed to fetch endpoints for dns record [%s]: endpoint is being removed", groomed)
			continue
		}
		newEndpointSubsets = append(newEndpointSubsets, targetEndpoint.Subsets...)
		targetEndpointUUID := fmt.Sprintf("%s/%s", targetEndpoint.Namespace, targetEndpoint.Name)
		targetEndpointUUIDs[targetEndpointUUID] = true
	}

	if toHandleExternalName {
		if externalName == "" {
			logrus.Infof("Deleting dns record [%s] HostName, externalName empty", obj.Name)
			if err := c.services.DeleteNamespaced(obj.Namespace, obj.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "Error deleting dns record [%s]", obj.Name)
			}
		} else if obj.Spec.Type == SvcTypeExternalName &&
			obj.Spec.ExternalName == externalName &&
			obj.Spec.ClusterIP == "" {
			logrus.Infof("HostName dns record [%s] up to date", obj.Name)
		} else {
			svcAlias := obj.DeepCopy()
			svcAlias.Spec.Type = SvcTypeExternalName
			svcAlias.Spec.ExternalName = externalName
			svcAlias.Spec.ClusterIP = ""
			logrus.Infof("Updating HostName of dns record [%s] to %s", obj.Name, externalName)
			if _, err := c.services.Update(svcAlias); err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "Error updating dns record [%s]", obj.Name)
			}
		}
		return nil
	}

	dnsServiceUUIDToTargetEndpointUUIDs.Store(key, targetEndpointUUIDs)

	ep, err := c.endpointLister.Get(obj.Namespace, obj.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "Failed to fetch endpoints for DNSRecord service [%s] in namespace [%s]", obj.Name, obj.Namespace)
	}

	if ep == nil {
		controller := true
		ownerRef := metav1.OwnerReference{
			Name:       obj.Name,
			APIVersion: "v1",
			UID:        obj.UID,
			Kind:       "Service",
			Controller: &controller,
		}

		ep := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:            obj.Name,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
				Namespace:       obj.Namespace,
			},
			Subsets: newEndpointSubsets,
		}
		logrus.Infof("Creating endpoints for targetDnsRecordIds service [%s]: %v", key, ep.Subsets)
		if _, err := c.endpoints.Create(ep); err != nil {
			return err
		}
	} else {
		if reflect.DeepEqual(ep.Subsets, newEndpointSubsets) {
			logrus.Debugf("Endpoints are up to date for DNSRecord service [%s]", obj.Name)
			return nil
		}
		logrus.Infof("Updating endpoints for DNSRecord service [%s]. Old: [%v], new: [%v]", obj.Name, ep.Subsets, newEndpointSubsets)
		toUpdate := ep.DeepCopy()
		toUpdate.Subsets = newEndpointSubsets
		_, err = c.endpoints.Update(toUpdate)
		if err != nil {
			return errors.Wrapf(err, "Failed to update endpoint for DNSRecord service [%s]", obj.Name)
		}
	}

	return nil
}

func (c *EndpointController) reconcileServicesForEndpoint(key string, obj *corev1.Endpoints) error {
	var dnsRecordServicesToReconcile []string
	dnsServiceUUIDToTargetEndpointUUIDs.Range(func(k, v interface{}) bool {
		if _, ok := v.(map[string]bool)[key]; ok {
			dnsRecordServicesToReconcile = append(dnsRecordServicesToReconcile, k.(string))
		}
		return true
	})

	for _, dnsRecordServiceToReconcile := range dnsRecordServicesToReconcile {
		splitted := strings.Split(dnsRecordServiceToReconcile, "/")
		namespace := splitted[0]
		serviceName := splitted[1]
		c.serviceController.Enqueue(namespace, serviceName)
	}

	return nil
}

func (c *Controller) queueUpdateForExtNameSvc(key string, delete bool) {
	alias, ok := serviceUUIDToHostNameAlias.Load(key)
	if ok && delete {
		serviceUUIDToHostNameAlias.Delete(key)
	}
	if ok {
		splitted := strings.Split(convert.ToString(alias), "/")
		namespace := splitted[0]
		aliasName := splitted[1]
		c.serviceController.Enqueue(namespace, aliasName)
	}
}

func (c *Controller) checkLinkedSvc(namespace string, service string, original string) (bool, string, bool, error) {
	svc, err := c.serviceLister.Get(namespace, service)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, "", false, nil
		}
		return false, "", false, err
	}
	if svc.Spec.Type == SvcTypeExternalName {
		if svc.DeletionTimestamp != nil {
			return true, "", true, nil
		}
		return true, svc.Spec.ExternalName, false, nil
	}
	return false, original, false, nil
}
