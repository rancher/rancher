package targetworkloadservice

import (
	"context"
	"encoding/json"

	"fmt"

	"strings"

	"sync"

	"reflect"

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

var workloadServiceUUIDToWorkloadIDs sync.Map

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
	if obj == nil || obj.DeletionTimestamp != nil {
		if value, ok := workloadServiceUUIDToWorkloadIDs.Load(key); ok {
			if err := c.updateServiceWorkloadPods(key, value.(map[string]bool)); err != nil {
				return err
			}
		}
		// delete from the workload map
		workloadServiceUUIDToWorkloadIDs.Delete(key)
		return nil
	}

	workloadIDs := getServiceWorkloadIDs(obj)
	// update pods (if needed) with service selector labels
	targetWorkloadIDs, err := c.reconcilePods(key, obj, workloadIDs)
	if err != nil {
		return err
	}

	// if workloadIDs changed, push update for all the pods, so they reconcile the labels
	workloadIDsToUpdate := map[string]bool{}
	oldMap, ok := workloadServiceUUIDToWorkloadIDs.Load(key)
	if ok {
		for workloadID := range oldMap.(map[string]bool) {
			workloadIDsToUpdate[workloadID] = true
		}
	}
	for workloadID := range targetWorkloadIDs {
		workloadIDsToUpdate[workloadID] = true
	}

	if err := c.updateServiceWorkloadPods(key, workloadIDsToUpdate); err != nil {
		return err
	}

	//reset the map
	workloadServiceUUIDToWorkloadIDs.Store(key, targetWorkloadIDs)

	return nil
}

func getServiceWorkloadIDs(obj *corev1.Service) []string {
	var workloadIDs []string
	if obj.Annotations == nil {
		return workloadIDs
	}
	value, ok := obj.Annotations[util.WorkloadAnnotation]
	if !ok || value == "" {
		return workloadIDs
	}
	noop, ok := obj.Annotations[util.WorkloadAnnotatioNoop]
	if ok && noop == "true" {
		return workloadIDs
	}

	err := json.Unmarshal([]byte(value), &workloadIDs)
	if err != nil {
		// just log the error, can't really do anything here.
		logrus.Debugf("Failed to unmarshal targetWorkloadIds", err)
	}
	return workloadIDs
}

func (c *Controller) updateServiceWorkloadPods(key string, workloadIDsToCleanup map[string]bool) error {
	if len(workloadIDsToCleanup) == 0 {
		return nil
	}
	var podsToEnqueue []*corev1.Pod
	var workloadsToCleanup []*util.Workload
	for workloadID := range workloadIDsToCleanup {
		workload, err := c.workloadLister.GetByWorkloadID(workloadID)
		if err != nil || workload == nil {
			logrus.Warnf("Failed to fetch workload [%s]: [%v]", workloadID, err)
			continue
		}

		pods, err := c.getPodsForWorkload(workload)
		if err != nil {
			return err
		}
		podsToEnqueue = append(podsToEnqueue, pods...)
		workloadsToCleanup = append(workloadsToCleanup, workload)
	}

	for _, pod := range podsToEnqueue {
		c.pods.Controller().Enqueue(pod.Namespace, pod.Name)
	}

	for _, workload := range workloadsToCleanup {
		c.workloadLister.EnqueueWorkload(workload)
	}
	return nil
}

func (c *Controller) reconcilePods(key string, obj *corev1.Service, workloadIDs []string) (map[string]bool, error) {
	if len(workloadIDs) == 0 {
		return nil, nil
	}
	if obj.Spec.Selector == nil {
		obj.Spec.Selector = map[string]string{}
	}
	selectorToAdd := getServiceSelector(obj.Name)
	var toUpdate *corev1.Service
	if _, ok := obj.Spec.Selector[selectorToAdd]; !ok {
		toUpdate = obj.DeepCopy()
		toUpdate.Spec.Selector[selectorToAdd] = "true"
		_, err := c.services.Update(toUpdate)
		if err != nil {
			return nil, err
		}
	}
	return c.updatePods(key, obj, workloadIDs)
}

func (c *Controller) getPodsForWorkload(workload *util.Workload) ([]*corev1.Pod, error) {
	set := labels.Set{}
	for key, val := range workload.SelectorLabels {
		set[key] = val
	}
	workloadSelector := labels.SelectorFromSet(set)
	return c.podLister.List(workload.Namespace, workloadSelector)
}

func (c *Controller) updatePods(serviceName string, obj *corev1.Service, workloadIDs []string) (map[string]bool, error) {
	var podsToUpdate []*corev1.Pod
	targetWorkloadIDs := map[string]bool{}
	for _, workloadID := range workloadIDs {
		targetWorkload, err := c.workloadLister.GetByWorkloadID(workloadID)
		if err != nil || targetWorkload == nil {
			logrus.Warnf("Failed to fetch workload [%s]: [%v]", workloadID, err)
			continue
		}

		pods, err := c.getPodsForWorkload(targetWorkload)
		if err != nil {
			return nil, err
		}

		// Add workload/deployment to the system map
		targetWorkloadIDs[workloadID] = true

		// Find all the pods satisfying deployments' selectors
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
	}

	// Update the pods with the label
	for _, pod := range podsToUpdate {
		toUpdate := pod.DeepCopy()
		for svcSelectorKey, svcSelectorValue := range obj.Spec.Selector {
			toUpdate.Labels[svcSelectorKey] = svcSelectorValue
		}
		if _, err := c.pods.Update(toUpdate); err != nil {
			return nil, errors.Wrapf(err, "Failed to update pod [%s] with workload service selector [%s]",
				pod.Name, fmt.Sprintf("%s/%s", obj.Namespace, obj.Name))
		}
	}
	return targetWorkloadIDs, nil
}

func getServiceSelector(serviceName string) string {
	return fmt.Sprintf("%s_%s", WorkloadIDLabelPrefix, serviceName)
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

	workloadServiceUUIDsToAdd := map[string]bool{}
	for _, d := range workloads {
		workloadServiceUUIDToWorkloadIDs.Range(func(k, v interface{}) bool {
			if _, ok := v.(map[string]bool)[d.Key]; ok {
				workloadServiceUUIDsToAdd[k.(string)] = true
			}
			return true
		})
	}

	workloadServicesLabels := map[string]string{}
	for workloadServiceUUID := range workloadServiceUUIDsToAdd {
		parts := strings.Split(workloadServiceUUID, "/")
		workloadService, err := c.serviceLister.Get(parts[0], parts[1])
		if err != nil || workloadService == nil {
			logrus.Warnf("Failed to fetch service [%s]: [%v]", workloadService, err)
			continue
		}

		for key, value := range workloadService.Spec.Selector {
			workloadServicesLabels[key] = value
		}
	}

	// remove old labels
	labels := map[string]string{}
	for key, value := range obj.Labels {
		if strings.HasPrefix(key, WorkloadIDLabelPrefix) {
			if _, ok := workloadServicesLabels[key]; !ok {
				continue
			}
		}
		labels[key] = value
	}

	// add new labels
	for key, value := range workloadServicesLabels {
		labels[key] = value
	}

	if reflect.DeepEqual(obj.Labels, labels) {
		return nil
	}
	toUpdate := obj.DeepCopy()
	toUpdate.Labels = labels
	_, err = c.pods.Update(toUpdate)
	return err
}
