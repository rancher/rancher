package servicemonitor

import (
	"fmt"
	"strings"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	util "github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func (c *MetricsServiceController) ensureService(key string, svc *corev1.Service) (runtime.Object, error) {
	if svc == nil || svc.DeletionTimestamp != nil {
		parts := strings.Split(key, "/")
		sms, err := c.smLister.List(parts[0], labels.NewSelector())
		if err != nil {
			return svc, err
		}
		for _, sm := range sms {
			c.smClient.Controller().Enqueue(sm.Namespace, sm.Name)
		}
		return svc, nil
	}

	if _, ok := svc.Annotations[metricsServiceLabel]; !ok {
		return svc, nil
	}

	var owner *monitoringv1.ServiceMonitor
	var err error
	for _, o := range svc.OwnerReferences {
		if o.Kind == "ServiceMonitor" {
			owner, err = c.smLister.Get(svc.Namespace, o.Name)
			if err != nil {
				return svc, err
			}
		}
	}
	ports := GetServicePortsFromEndpoint(owner.Spec.Endpoints)
	if !util.ArePortsEqual(ports, svc.Spec.Ports) {
		c.smClient.Controller().Enqueue(owner.Namespace, owner.Name)
	}

	return svc, nil
}

func (c *MetricsServiceController) ensureServiceMonitor(key string, obj *monitoringv1.ServiceMonitor) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		parts := strings.Split(key, "/")
		return obj, c.workloadLister.EnqueueAllWorkloads(parts[0])
	}

	var workload *util.Workload
	var err error
	for _, owner := range obj.OwnerReferences {
		key := fmt.Sprintf("%s:%s:%s", strings.ToLower(owner.Kind), obj.Namespace, owner.Name)
		workload, err = c.workloadLister.GetByWorkloadID(key)
		if err != nil {
			continue
		}
	}
	if workload == nil {
		return obj, nil
	}
	c.workloadLister.EnqueueWorkload(workload)
	return obj, nil
}
