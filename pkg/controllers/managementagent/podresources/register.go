package podresources

import (
	"context"
	"encoding/json"
	"time"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	RequestsAnnotation = "management.cattle.io/pod-requests"
	LimitsAnnotation   = "management.cattle.io/pod-limits"
)

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	p := podResources{
		podLister: workload.Core.Pods("").Controller().Lister(),
		nodes:     workload.Core.Nodes(""),
	}

	workload.Core.Nodes("").AddHandler(ctx, "podresource", p.onChange)
}

type podResources struct {
	podLister v1.PodLister
	nodes     v1.NodeInterface
}

func (p *podResources) onChange(key string, node *corev1.Node) (runtime.Object, error) {
	if node == nil {
		return node, nil
	}

	p.nodes.Controller().EnqueueAfter("", node.Name, 15*time.Second)

	pods, err := p.getNonTerminatedPods(node)
	if err != nil {
		return nil, err
	}

	requests, limits, err := getPodResourceAnnotations(pods)
	if err != nil {
		return nil, err
	}

	if node.Annotations[RequestsAnnotation] != requests ||
		node.Annotations[LimitsAnnotation] != limits {
		node := node.DeepCopy()
		if node.Annotations == nil {
			node.Annotations = map[string]string{}
		}
		node.Annotations[RequestsAnnotation] = requests
		node.Annotations[LimitsAnnotation] = limits
		return p.nodes.Update(node)
	}

	return node, nil
}

func (p *podResources) getNonTerminatedPods(node *corev1.Node) ([]*corev1.Pod, error) {
	var pods []*corev1.Pod
	fromCache, err := p.podLister.List("", labels.NewSelector())
	if err != nil {
		return pods, err
	}

	for _, pod := range fromCache {
		if pod.Spec.NodeName != node.Name {
			continue
		}
		// kubectl uses this cache to filter out the pods
		if pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed" {
			continue
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func getPodResourceAnnotations(pods []*corev1.Pod) (string, string, error) {
	requests, limits := aggregateRequestAndLimitsForNode(pods)
	requestsBytes, err := json.Marshal(requests)
	if err != nil {
		return "", "", err
	}

	limitsBytes, err := json.Marshal(limits)
	return string(requestsBytes), string(limitsBytes), err
}

func aggregateRequestAndLimitsForNode(pods []*corev1.Pod) (map[corev1.ResourceName]resource.Quantity, map[corev1.ResourceName]resource.Quantity) {
	requests, limits := map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, pod := range pods {
		podRequests, podLimits := getPodData(pod)
		addMap(podRequests, requests)
		addMap(podLimits, limits)
	}
	if pods != nil {
		requests[corev1.ResourcePods] = *resource.NewQuantity(int64(len(pods)), resource.DecimalSI)
	}
	return requests, limits
}

func getPodData(pod *corev1.Pod) (map[corev1.ResourceName]resource.Quantity, map[corev1.ResourceName]resource.Quantity) {
	requests, limits := map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, container := range pod.Spec.Containers {
		addMap(container.Resources.Requests, requests)
		addMap(container.Resources.Limits, limits)
	}

	for _, container := range pod.Spec.InitContainers {
		addMapForInit(container.Resources.Requests, requests)
		addMapForInit(container.Resources.Limits, limits)
	}
	return requests, limits
}

func addMap(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		if value, ok := data2[name]; !ok {
			data2[name] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			data2[name] = value
		}
	}
}

func addMapForInit(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		value, ok := data2[name]
		if !ok {
			data2[name] = quantity.DeepCopy()
			continue
		}
		if quantity.Cmp(value) > 0 {
			data2[name] = quantity.DeepCopy()
		}
	}
}
