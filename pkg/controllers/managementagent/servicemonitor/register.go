package servicemonitor

import (
	"context"

	util "github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	rmonitoringv1 "github.com/rancher/rancher/pkg/generated/norman/monitoring.coreos.com/v1"
	"github.com/rancher/rancher/pkg/types/config"
)

const (
	creatorIDAnnotation  = "field.cattle.io/creatorId"
	metricsAnnotation    = "field.cattle.io/workloadMetrics"
	metricsServiceLabel  = "cattle.io/metrics"
	metricsServiceSuffix = "-metrics"
	servicesAnnotation   = "field.cattle.io/serviceIDs"
)

/*
The MetricsServiceController maintains the relation between workload, service monitor and service.
The service monitor will create services that point to workloads or pods with selector. The service monitor
owns those service and should delete with the object.
*/
type MetricsServiceController struct {
	serviceLister  v1.ServiceLister
	services       v1.ServiceInterface
	smLister       rmonitoringv1.ServiceMonitorLister
	smClient       rmonitoringv1.ServiceMonitorInterface
	workloadLister util.CommonController
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &MetricsServiceController{
		serviceLister:  workload.Core.Services("").Controller().Lister(),
		services:       workload.Core.Services(""),
		smLister:       workload.Monitoring.ServiceMonitors("").Controller().Lister(),
		smClient:       workload.Monitoring.ServiceMonitors(""),
		workloadLister: util.NewWorkloadController(ctx, workload, nil),
	}
	util.NewWorkloadController(ctx, workload, c.createService)
	workload.Core.Services("").Controller().AddHandler(ctx, "workloadMetrics", c.ensureService)
	workload.Monitoring.ServiceMonitors("").Controller().AddHandler(ctx, "ensureServiceMetrics", c.ensureServiceMonitor)
	workload.Monitoring.ServiceMonitors("").Controller().AddHandler(ctx, "workloadMetrics", c.syncServiceMonitor)
}
