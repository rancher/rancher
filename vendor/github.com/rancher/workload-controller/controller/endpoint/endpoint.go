package endpoint

import (
	"context"
	"strings"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/rancher/workload-controller/controller/dnsrecord"
	corev1 "k8s.io/api/core/v1"
)

// This controller is responsible for monitoring endpoints
// finding out if they are the part of DNSRecord service
// and calling the update on the target service

type Controller struct {
	serviceController v1.ServiceController
	serviceLister     v1.ServiceLister
}

func Register(ctx context.Context, workload *config.WorkloadContext) {
	c := &Controller{
		serviceController: workload.Core.Services("").Controller(),
		serviceLister:     workload.Core.Services("").Controller().Lister(),
	}
	workload.Core.Endpoints("").AddHandler(c.GetName(), c.reconcileServicesForEndpoint)
}

func (c *Controller) GetName() string {
	return "dnsRecordEndpointsController"
}

func (c *Controller) reconcileServicesForEndpoint(key string, obj *corev1.Endpoints) error {
	var dnsRecordServicesToReconcile []string
	dnsrecord.ServiceUUIDToTargetEndpointUUIDs.Range(func(k, v interface{}) bool {
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
