package monitoring

import (
	"encoding/json"
	"reflect"
	"sort"

	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	apiv1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	rmonitoringv1 "github.com/rancher/rancher/pkg/generated/norman/monitoring.coreos.com/v1"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	projectNSAnnotation        = "project.cattle.io/namespaces"
	promByMemberNamespaceIndex = "monitoring.cluster.cattle.io/prom-by-member-ns"
)

type ConfigRefreshHandler struct {
	prometheusClient  rmonitoringv1.PrometheusInterface
	prometheusIndexer cache.Indexer
	nsLister          apiv1.NamespaceLister
}

func (h *ConfigRefreshHandler) syncNamespace(key string, obj *corev1.Namespace) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}
	promList, err := h.prometheusIndexer.ByIndex(promByMemberNamespaceIndex, obj.Name)
	if err != nil {
		return obj, err
	}

	for _, object := range promList {
		prometheus, ok := object.(*monitoringv1.Prometheus)
		if !ok {
			continue
		}
		h.prometheusClient.Controller().Enqueue(prometheus.Namespace, prometheus.Name)
	}

	return obj, nil
}

func (h *ConfigRefreshHandler) syncPrometheus(key string, obj *monitoringv1.Prometheus) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	ns, err := h.nsLister.Get("", obj.Namespace)
	if err != nil {
		return obj, err
	}

	if len(ns.Labels) == 0 {
		return obj, nil
	}
	projectID, ok := ns.Labels[nslabels.ProjectIDFieldLabel]
	if !ok {
		return obj, nil
	}

	namespaces, err := h.getProjectNamespaces(projectID)
	if err != nil {
		return obj, err
	}

	annotationNamespaces, err := getAnnotationNamespaces(obj)
	if err != nil {
		return obj, err
	}

	if !reflect.DeepEqual(namespaces, annotationNamespaces) {
		newObj := obj.DeepCopy()
		if newObj.Annotations == nil {
			newObj.Annotations = make(map[string]string)
		}
		data, err := json.Marshal(namespaces)
		if err != nil {
			return obj, err
		}

		newObj.Annotations[projectNSAnnotation] = string(data)
		return h.prometheusClient.Update(newObj)
	}

	return obj, nil
}

func (h *ConfigRefreshHandler) getProjectNamespaces(projectID string) ([]string, error) {
	nsList, err := h.nsLister.List("", labels.Set(map[string]string{
		nslabels.ProjectIDFieldLabel: projectID,
	}).AsSelector())
	if err != nil {
		return nil, err
	}
	var rtn []string
	for _, ns := range nsList {
		rtn = append(rtn, ns.Name)
	}

	sort.Strings(rtn)
	return rtn, nil
}

func getAnnotationNamespaces(obj *monitoringv1.Prometheus) ([]string, error) {
	var rtn []string
	data, ok := obj.Annotations[projectNSAnnotation]
	if !ok {
		return rtn, nil
	}

	if err := json.Unmarshal([]byte(data), &rtn); err != nil {
		logrus.WithError(err).Warn("unmarshal json data from prometheus crd annotation")
		return rtn, err
	}

	sort.Strings(rtn)
	return rtn, nil
}

func promsByMemberNamespace(obj interface{}) ([]string, error) {
	p, ok := obj.(*monitoringv1.Prometheus)
	if !ok {
		return []string{}, nil
	}
	return getAnnotationNamespaces(p)
}
