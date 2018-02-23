package workloadservice

import (
	"context"

	"fmt"

	"strings"

	"sync"

	"github.com/pkg/errors"
	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring services with targetWorkloadIds,
// locating corresponding pods, and marking them with the label to satisfy service selector

const (
	WorkloadIDLabelPrefix = "workloadID"
)

var workloadServiceUUIDToDeploymentUUIDs sync.Map

type Controller struct {
	pods            v1.PodInterface
	workloadLister  util.CommonController
	podLister       v1.PodLister
	namespaceLister v1.NamespaceLister
	serviceLister   v1.ServiceLister
	services        v1.ServiceInterface
}

type PodController struct {
	pods           v1.PodInterface
	workloadLister util.CommonController
	serviceLister  v1.ServiceLister
	services       v1.ServiceInterface
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &Controller{
		pods:            workload.Core.Pods(""),
		workloadLister:  util.NewWorkloadController(workload, nil),
		podLister:       workload.Core.Pods("").Controller().Lister(),
		namespaceLister: workload.Core.Namespaces("").Controller().Lister(),
		serviceLister:   workload.Core.Services("").Controller().Lister(),
		services:        workload.Core.Services(""),
	}
	p := &PodController{
		workloadLister: util.NewWorkloadController(workload, nil),
		pods:           workload.Core.Pods(""),
		serviceLister:  workload.Core.Services("").Controller().Lister(),
		services:       workload.Core.Services(""),
	}
	workload.Core.Services("").AddHandler("workloadServiceController", c.sync)
	workload.Core.Pods("").AddHandler("podToWorkloadServiceController", p.sync)
}

func (c *Controller) sync(key string, obj *corev1.Service) error {
	if obj == nil {
		// delete from the workload map
		workloadServiceUUIDToDeploymentUUIDs.Delete(key)
		return nil
	}

	return c.reconcilePods(key, obj)
}

func (c *Controller) reconcilePods(key string, obj *corev1.Service) error {
	if obj.Annotations == nil {
		return nil
	}
	value, ok := obj.Annotations[util.WorkloadAnnotation]
	if !ok || value == "" {
		return nil
	}
	workdloadIDs := strings.Split(value, ",")

	if obj.Spec.Selector == nil {
		obj.Spec.Selector = make(map[string]string)
	}
	selectorToAdd := getServiceSelector(obj)
	var toUpdate *corev1.Service
	if _, ok := obj.Spec.Selector[selectorToAdd]; !ok {
		toUpdate = obj.DeepCopy()
		toUpdate.Spec.Selector[selectorToAdd] = "true"
	}
	if err := c.updatePods(key, obj, workdloadIDs); err != nil {
		return err
	}
	if toUpdate != nil {
		_, err := c.services.Update(toUpdate)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) updatePods(serviceName string, obj *corev1.Service, workloadIDs []string) error {
	var podsToUpdate []*corev1.Pod
	set := labels.Set{}
	for key, val := range obj.Spec.Selector {
		set[key] = val
	}
	// reset the map
	targetWorkloadUUIDs := make(map[string]bool)
	for _, workloadID := range workloadIDs {
		targetWorkload, err := c.workloadLister.GetByWorkloadID(workloadID)
		if err != nil {
			logrus.Warnf("Failed to fetch workload [%s]: [%v]", workloadID, err)
			continue
		}

		// Add workload/deployment to the system map
		targetWorkloadUUID := fmt.Sprintf("%s/%s", targetWorkload.Namespace, targetWorkload.Name)
		targetWorkloadUUIDs[targetWorkloadUUID] = true

		// Find all the pods satisfying deployments' selectors
		set := labels.Set{}
		for key, val := range targetWorkload.SelectorLabels {
			set[key] = val
		}
		workloadSelector := labels.SelectorFromSet(set)
		pods, err := c.podLister.List(targetWorkload.Namespace, workloadSelector)
		if err != nil {
			return errors.Wrapf(err, "Failed to list pods for target workload [%s]", workloadID)
		}
		for _, pod := range pods {
			if pod.DeletionTimestamp != nil {
				continue
			}
			for svsSelectorKey, svcSelectorValue := range obj.Spec.Selector {
				if value, ok := pod.Labels[svsSelectorKey]; ok && value == svcSelectorValue {
					continue
				}
				podsToUpdate = append(podsToUpdate, pod)
			}
		}

		// Update the pods with the label
		for _, pod := range podsToUpdate {
			toUpdate := pod.DeepCopy()
			for svcSelectorKey, svcSelectorValue := range obj.Spec.Selector {
				toUpdate.Labels[svcSelectorKey] = svcSelectorValue
			}
			if _, err := c.pods.Update(toUpdate); err != nil {
				return errors.Wrapf(err, "Failed to update pod [%s] for target workload [%s]", pod.Name, workloadID)
			}
		}
	}
	workloadServiceUUIDToDeploymentUUIDs.Store(serviceName, targetWorkloadUUIDs)
	return nil
}

func getServiceSelector(obj *corev1.Service) string {
	return fmt.Sprintf("%s_%s", WorkloadIDLabelPrefix, obj.Name)
}

func (c *PodController) sync(key string, obj *corev1.Pod) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	// filter out deployments that are match for the pods
	workloads, err := c.workloadLister.GetWorkloadsMatchingLabels(obj.Namespace, obj.Labels)
	if err != nil {
		return err
	}

	workloadServiceUUIDToAdd := []string{}
	for _, d := range workloads {
		deploymentUUID := fmt.Sprintf("%s/%s", d.Namespace, d.Name)
		workloadServiceUUIDToDeploymentUUIDs.Range(func(k, v interface{}) bool {
			if _, ok := v.(map[string]bool)[deploymentUUID]; ok {
				workloadServiceUUIDToAdd = append(workloadServiceUUIDToAdd, k.(string))
			}
			return true
		})
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
