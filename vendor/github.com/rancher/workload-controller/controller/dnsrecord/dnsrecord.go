package dnsrecord

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"sync"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	DNSAnnotation       = "field.cattle.io/targetDNSRecordIds"
	ProjectIDAnnotation = "field.cattle.io/projectId"
)

var dnsServiceUUIDToTargetEndpointUUIDs sync.Map

// Controller is responsible for monitoring DNSRecord services
// and populating the endpoint based on target service endpoints.
// The controller DOES NOT monitor the changes to the target endpoints;
// that would be handled in the by EndpointController
type Controller struct {
	endpoints       v1.EndpointsInterface
	endpointLister  v1.EndpointsLister
	namespaceLister v1.NamespaceLister
}

// EndpointController is responsible for monitoring endpoints
// finding out if they are the part of DNSRecord service
// and calling the update on the target service
type EndpointController struct {
	serviceController v1.ServiceController
	serviceLister     v1.ServiceLister
}

func Register(ctx context.Context, workload *config.WorkloadContext) {
	c := &Controller{
		endpoints:       workload.Core.Endpoints(""),
		endpointLister:  workload.Core.Endpoints("").Controller().Lister(),
		namespaceLister: workload.Core.Namespaces("").Controller().Lister(),
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
		dnsServiceUUIDToTargetEndpointUUIDs.Delete(key)
		return nil
	}
	return c.reconcileEndpoints(key, obj)
}

func getNamespaceProjectID(ns *corev1.Namespace) string {
	parts := strings.Split(ns.Annotations[ProjectIDAnnotation], ":")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func (c *Controller) reconcileEndpoints(key string, obj *corev1.Service) error {
	// only process services having targetDNSRecordIds in annotation
	if obj.Annotations == nil {
		return nil
	}
	value, ok := obj.Annotations[DNSAnnotation]
	if !ok {
		return nil
	}

	records := strings.Split(value, ",")
	var newEndpointSubsets []corev1.EndpointSubset
	targetEndpointUUIDs := make(map[string]bool)
	// filter out project namespaces
	namespaces, err := GetProjectNamespaces(c.namespaceLister, obj)
	if err != nil {
		return err
	}
	for _, record := range records {
		groomed := strings.TrimSpace(record)
		namespaceService := strings.Split(groomed, ":")
		if len(namespaceService) < 2 {
			return fmt.Errorf("Wrong format for dns record [%s]", groomed)
		}
		namespace := namespaceService[0]
		if _, ok := namespaces[namespace]; !ok {
			logrus.Warnf("Failed to find namespace [%s] for DNSRecord service [%s]", namespace, groomed)
			continue
		}
		service := namespaceService[1]
		targetEndpoint, err := c.endpointLister.Get(namespace, service)
		if err != nil {
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
	dnsServiceUUIDToTargetEndpointUUIDs.Store(key, targetEndpointUUIDs)

	ep, err := c.endpointLister.Get(obj.Namespace, obj.Name)
	if err != nil {
		return errors.Wrapf(err, "Failed to fetch endpoints for DNSRecord service [%s] in namespace [%s]", obj.Name, obj.Namespace)
	}

	if reflect.DeepEqual(ep.Subsets, newEndpointSubsets) {
		logrus.Debugf("Endpoints are up to date for DNSRecord service [%s]", obj.Name)
		return nil
	}
	logrus.Infof("Updating endpoints for DNSREcord service [%s]. Old: [%v], new: [%v]", obj.Name, ep.Subsets, newEndpointSubsets)
	toUpdate := ep.DeepCopy()
	toUpdate.Subsets = newEndpointSubsets
	_, err = c.endpoints.Update(toUpdate)
	if err != nil {
		return errors.Wrapf(err, "Failed to update endpoint for DNSRecord service [%s]", obj.Name)
	}

	return nil
}

func GetProjectNamespaces(lister v1.NamespaceLister, obj *corev1.Service) (map[string]bool, error) {
	namespaces := make(map[string]bool)
	selfNamespace, err := lister.Get("", obj.Namespace)
	if err != nil {
		return namespaces, errors.Wrapf(err, "Failed to find endpoint for DNSRecord service [%s]", obj.Name)
	}
	if selfNamespace == nil {
		return namespaces, fmt.Errorf("Failed to find namespace for for DNSRecord service [%s]", obj.Name)
	}

	selfProjectID := getNamespaceProjectID(selfNamespace)
	if selfProjectID == "" {
		return namespaces, fmt.Errorf("Failed to get projectID for for DNSRecord service [%s]", obj.Name)
	}

	nss, err := lister.List("", labels.NewSelector())
	if err != nil {
		return namespaces, errors.Wrapf(err, "Failed to fetch namespaces as a part of DNSRecord service [%s] reconcilation", obj.Name)
	}
	for _, ns := range nss {
		if getNamespaceProjectID(ns) == selfProjectID {
			namespaces[ns.Name] = true
		}
	}
	return namespaces, nil
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
