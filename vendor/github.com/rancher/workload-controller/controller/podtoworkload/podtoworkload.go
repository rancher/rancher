package podtoworkload

import (
	"context"

	"fmt"

	"strings"

	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/rancher/workload-controller/controller/workloadservice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Controller struct {
	pods             v1.PodInterface
	deploymentLister v1beta2.DeploymentLister
	serviceLister    v1.ServiceLister
}

func Register(ctx context.Context, workload *config.WorkloadContext) {
	c := &Controller{
		deploymentLister: workload.Apps.Deployments("").Controller().Lister(),
		serviceLister:    workload.Core.Services("").Controller().Lister(),
		pods:             workload.Core.Pods(""),
	}
	workload.Core.Pods("").AddHandler(c.GetName(), c.sync)
}

func (c *Controller) sync(key string, obj *corev1.Pod) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	// filter out deployments that are match for the pod
	deployments, err := c.deploymentLister.List(obj.Namespace, labels.NewSelector())
	if err != nil {
		return err
	}

	workloadServiceUUIDToAdd := []string{}
	for _, d := range deployments {
		selector := labels.SelectorFromSet(d.Spec.Selector.MatchLabels)
		if selector.Matches(labels.Set(obj.Labels)) {
			deploymentUUID := fmt.Sprintf("%s/%s", d.Namespace, d.Name)
			workloadservice.WorkloadServiceUUIDToDeploymentUUIDs.Range(func(k, v interface{}) bool {
				if _, ok := v.(map[string]bool)[deploymentUUID]; ok {
					workloadServiceUUIDToAdd = append(workloadServiceUUIDToAdd, k.(string))
				}
				return true
			})
		}
	}

	workloadServicesLabels := make(map[string]string)
	for _, workloadServiceUUID := range workloadServiceUUIDToAdd {
		splitted := strings.Split(workloadServiceUUID, "/")
		workload, err := c.serviceLister.Get(obj.Namespace, splitted[1])
		if err != nil {
			return err
		}
		for key, value := range workload.Spec.Selector {
			workloadServicesLabels[key] = value
		}
	}
	if len(workloadServicesLabels) == 0 {
		return nil
	}
	toUpdate := obj.DeepCopy()
	for key, value := range workloadServicesLabels {
		toUpdate.Labels[key] = value
	}
	_, err = c.pods.Update(toUpdate)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) GetName() string {
	return "podToWorkloadServiceController"
}
