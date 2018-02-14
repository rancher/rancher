package util

import (
	"reflect"
	"strings"

	"fmt"

	"strconv"

	"github.com/rancher/types/apis/apps/v1beta2"
	batchv1 "github.com/rancher/types/apis/batch/v1"
	"github.com/rancher/types/apis/batch/v1beta1"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var WorkloadKinds = map[string]bool{
	"Deployment":            true,
	"ReplicationController": true,
	"ReplicaSet":            true,
	"DaemonSet":             true,
	"StatefulSet":           true,
	"Job":                   true,
	"CronJob":               true,
}

const (
	AppVersion         = "apps/v1beta2"
	BatchBetaVersion   = "batch/v1beta1"
	BatchVersion       = "batch/v1"
	WorkloadAnnotation = "field.cattle.io/targetWorkloadIds"
)

type Workload struct {
	Name            string
	Namespace       string
	UUID            types.UID
	SelectorLabels  map[string]string
	Annotations     map[string]string
	TemplateSpec    *corev1.PodTemplateSpec
	Kind            string
	APIVersion      string
	OwnerReferences []metav1.OwnerReference
}

type WorkloadLister struct {
	DeploymentLister            v1beta2.DeploymentLister
	ReplicationControllerLister v1.ReplicationControllerLister
	ReplicaSetLister            v1beta2.ReplicaSetLister
	DaemonSetLister             v1beta2.DaemonSetLister
	StatefulSetLister           v1beta2.StatefulSetLister
	JobLister                   batchv1.JobLister
	CronJobLister               v1beta1.CronJobLister
	ServiceLister               v1.ServiceLister
	Services                    v1.ServiceInterface
}

func NewWorkloadLister(workload *config.UserOnlyContext) WorkloadLister {
	return WorkloadLister{
		DeploymentLister:            workload.Apps.Deployments("").Controller().Lister(),
		ReplicationControllerLister: workload.Core.ReplicationControllers("").Controller().Lister(),
		ReplicaSetLister:            workload.Apps.ReplicaSets("").Controller().Lister(),
		DaemonSetLister:             workload.Apps.DaemonSets("").Controller().Lister(),
		StatefulSetLister:           workload.Apps.StatefulSets("").Controller().Lister(),
		JobLister:                   workload.BatchV1.Jobs("").Controller().Lister(),
		CronJobLister:               workload.BatchV1Beta1.CronJobs("").Controller().Lister(),
		ServiceLister:               workload.Core.Services("").Controller().Lister(),
		Services:                    workload.Core.Services(""),
	}
}

func (w WorkloadLister) GetByName(key string) (*Workload, error) {
	splitted := strings.Split(key, ":")
	if len(splitted) != 3 {
		return nil, fmt.Errorf("workload name [%s] is invalid", key)
	}
	workloadType := strings.ToLower(splitted[0])
	namespace := splitted[1]
	name := splitted[2]
	var workload *Workload
	switch workloadType {
	case "replicationcontroller":
		o, err := w.ReplicationControllerLister.Get(namespace, name)
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}
		labelSelector := &metav1.LabelSelector{
			MatchLabels: o.Spec.Selector,
		}
		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, labelSelector, o.Annotations, o.Spec.Template, o.OwnerReferences)
	case "replicaset":
		o, err := w.ReplicaSetLister.Get(namespace, name)
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template, o.OwnerReferences)
	case "daemonset":
		o, err := w.DaemonSetLister.Get(namespace, name)
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template, o.OwnerReferences)
	case "statefulset":
		o, err := w.StatefulSetLister.Get(namespace, name)
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template, o.OwnerReferences)
	case "job":
		o, err := w.JobLister.Get(namespace, name)
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}
		labelSelector := &metav1.LabelSelector{
			MatchLabels: o.Spec.Selector.MatchLabels,
		}
		workload = getWorkload(namespace, name, workloadType, BatchVersion, o.UID, labelSelector, o.Annotations, &o.Spec.Template, o.OwnerReferences)
	case "cronjob":
		o, err := w.CronJobLister.Get(namespace, name)
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}
		labelSelector := &metav1.LabelSelector{
			MatchLabels: o.Spec.JobTemplate.Spec.Selector.MatchLabels,
		}
		workload = getWorkload(namespace, name, workloadType, BatchBetaVersion, o.UID, labelSelector, o.Annotations, &o.Spec.JobTemplate.Spec.Template, o.OwnerReferences)
	default:
		o, err := w.DeploymentLister.Get(namespace, name)
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, "deployment", AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template, o.OwnerReferences)
	}
	return workload, nil
}

func getWorkload(namespace string, name string, kind string, apiVersion string, UUID types.UID, selectorLabels *metav1.LabelSelector, annotations map[string]string, podTemplateSpec *corev1.PodTemplateSpec, ownerRefs []metav1.OwnerReference) *Workload {
	return &Workload{
		Name:            name,
		Namespace:       namespace,
		SelectorLabels:  getSelectorLables(selectorLabels),
		UUID:            UUID,
		Annotations:     annotations,
		TemplateSpec:    podTemplateSpec,
		OwnerReferences: ownerRefs,
		Kind:            kind,
		APIVersion:      apiVersion,
	}
}

func (w WorkloadLister) GetBySelectorMatch(namespace string, selectorLabels map[string]string) ([]*Workload, error) {
	var workloads []*Workload
	deployments, err := w.DeploymentLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, d := range deployments {
		selector := labels.SelectorFromSet(d.Spec.Selector.MatchLabels)
		if selector.Matches(labels.Set(selectorLabels)) {
			workloads = append(workloads, &Workload{
				Name:           d.Name,
				Namespace:      d.Namespace,
				SelectorLabels: getSelectorLables(d.Spec.Selector),
			})
		}
	}

	return workloads, nil
}

func getSelectorLables(s *metav1.LabelSelector) map[string]string {
	selectorLabels := make(map[string]string)
	for key, value := range s.MatchLabels {
		selectorLabels[key] = value
	}
	return selectorLabels
}

func (w WorkloadLister) GetServiceForWorkload(workload *Workload) (*corev1.Service, error) {
	services, err := w.ServiceLister.List(workload.Namespace, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	//TODO check for Rancher annotation on a workload before creating a headless service

	for _, s := range services {
		if s.DeletionTimestamp != nil {
			continue
		}
		if s.Spec.Type != "ClusterIP" {
			continue
		}
		if s.Spec.ClusterIP != "None" {
			continue
		}
		for _, ref := range s.OwnerReferences {
			if reflect.DeepEqual(ref.UID, workload.UUID) {
				return s, nil
			}
		}
	}
	return nil, nil
}

func (w *WorkloadLister) CreateServiceForWorkload(workload *Workload) error {
	// do not create if object is "owned" by other workload
	for _, o := range workload.OwnerReferences {
		if ok := WorkloadKinds[o.Kind]; ok {
			return nil
		}
	}

	s, err := w.GetServiceForWorkload(workload)
	if err != nil {
		return err
	}
	if s != nil {
		return nil
	}
	ownerRef := metav1.OwnerReference{
		Name:       workload.Name,
		APIVersion: workload.APIVersion,
		UID:        workload.UUID,
		Kind:       workload.Kind,
	}

	var servicePorts []corev1.ServicePort
	for _, c := range workload.TemplateSpec.Spec.Containers {
		for _, p := range c.Ports {
			var portName string
			if p.Name == "" {
				portName = fmt.Sprintf("%s-%s", strconv.FormatInt(int64(p.ContainerPort), 10), c.Name)
			} else {
				portName = fmt.Sprintf("%s-%s", p.Name, c.Name)
			}
			servicePort := corev1.ServicePort{
				Port:       p.ContainerPort,
				TargetPort: intstr.Parse(strconv.FormatInt(int64(p.ContainerPort), 10)),
				Protocol:   p.Protocol,
				Name:       portName,
			}
			servicePorts = append(servicePorts, servicePort)
		}
	}

	annotations := make(map[string]string)
	annotations[WorkloadAnnotation] = workload.getKey()

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "service-",
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Namespace:       workload.Namespace,
			Annotations:     annotations,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Type:      "ClusterIP",
			Ports:     servicePorts,
		},
	}
	logrus.Infof("Creating headless service for workload %s", workload.getKey())
	_, err = w.Services.Create(service)
	return err
}

func (wk Workload) getKey() string {
	return fmt.Sprintf("%s:%s:%s", wk.Kind, wk.Namespace, wk.Name)
}
