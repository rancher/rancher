package workload

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rancher/norman/types/convert"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	batchv1 "github.com/rancher/rancher/pkg/generated/norman/batch/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	k8sappv1 "k8s.io/api/apps/v1"
	corebatchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	AppVersion                         = "apps/v1beta2"
	BatchBetaVersion                   = "batch/v1beta1"
	BatchVersion                       = "batch/v1"
	WorkloadAnnotation                 = "field.cattle.io/targetWorkloadIds"
	PortsAnnotation                    = "field.cattle.io/ports"
	ClusterIPServiceType               = "ClusterIP"
	DeploymentType                     = "deployment"
	ReplicationControllerType          = "replicationcontroller"
	ReplicaSetType                     = "replicaset"
	DaemonSetType                      = "daemonset"
	StatefulSetType                    = "statefulset"
	JobType                            = "job"
	CronJobType                        = "cronjob"
	WorkloadAnnotatioNoop              = "workload.cattle.io/targetWorkloadIdNoop"
	WorkloaAnnotationdPortBasedService = "workload.cattle.io/workloadPortBased"
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
	Labels          map[string]string
	Key             string
	Status          *Status
}

type Status struct {
	Replicas          int32
	AvailableReplicas int32
	Conditions        []map[string]interface{}
}

type CommonController struct {
	DeploymentLister            appsv1.DeploymentLister
	ReplicationControllerLister v1.ReplicationControllerLister
	ReplicaSetLister            appsv1.ReplicaSetLister
	DaemonSetLister             appsv1.DaemonSetLister
	StatefulSetLister           appsv1.StatefulSetLister
	JobLister                   batchv1.JobLister
	CronJobLister               batchv1.CronJobLister
	Deployments                 appsv1.DeploymentInterface
	ReplicationControllers      v1.ReplicationControllerInterface
	ReplicaSets                 appsv1.ReplicaSetInterface
	DaemonSets                  appsv1.DaemonSetInterface
	StatefulSets                appsv1.StatefulSetInterface
	Jobs                        batchv1.JobInterface
	CronJobs                    batchv1.CronJobInterface
	Sync                        func(key string, w *Workload) error
}

func NewWorkloadController(ctx context.Context, workload *config.UserOnlyContext, f func(key string, w *Workload) error) CommonController {
	c := CommonController{
		DeploymentLister:            workload.Apps.Deployments("").Controller().Lister(),
		ReplicationControllerLister: workload.Core.ReplicationControllers("").Controller().Lister(),
		ReplicaSetLister:            workload.Apps.ReplicaSets("").Controller().Lister(),
		DaemonSetLister:             workload.Apps.DaemonSets("").Controller().Lister(),
		StatefulSetLister:           workload.Apps.StatefulSets("").Controller().Lister(),
		JobLister:                   workload.BatchV1.Jobs("").Controller().Lister(),
		CronJobLister:               workload.BatchV1.CronJobs("").Controller().Lister(),
		Deployments:                 workload.Apps.Deployments(""),
		ReplicationControllers:      workload.Core.ReplicationControllers(""),
		ReplicaSets:                 workload.Apps.ReplicaSets(""),
		DaemonSets:                  workload.Apps.DaemonSets(""),
		StatefulSets:                workload.Apps.StatefulSets(""),
		Jobs:                        workload.BatchV1.Jobs(""),
		CronJobs:                    workload.BatchV1.CronJobs(""),
		Sync:                        f,
	}
	if f != nil {
		workload.Apps.Deployments("").AddHandler(ctx, getName(), c.syncDeployments)
		workload.Core.ReplicationControllers("").AddHandler(ctx, getName(), c.syncReplicationControllers)
		workload.Apps.ReplicaSets("").AddHandler(ctx, getName(), c.syncReplicaSet)
		workload.Apps.DaemonSets("").AddHandler(ctx, getName(), c.syncDaemonSet)
		workload.Apps.StatefulSets("").AddHandler(ctx, getName(), c.syncStatefulSet)
		workload.BatchV1.Jobs("").AddHandler(ctx, getName(), c.syncJob)
		workload.BatchV1.CronJobs("").AddHandler(ctx, getName(), c.syncCronJob)
	}
	return c
}

func (c *CommonController) syncDeployments(key string, obj *k8sappv1.Deployment) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	w, err := c.getWorkload(key, DeploymentType)
	if err != nil || w == nil {
		return nil, err
	}

	return nil, c.Sync(key, w)
}

func (c *CommonController) syncReplicationControllers(key string, obj *corev1.ReplicationController) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	w, err := c.getWorkload(key, ReplicationControllerType)
	if err != nil || w == nil {
		return nil, err
	}
	return nil, c.Sync(key, w)
}

func (c *CommonController) syncReplicaSet(key string, obj *k8sappv1.ReplicaSet) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	w, err := c.getWorkload(key, ReplicaSetType)
	if err != nil || w == nil {
		return nil, err
	}
	return nil, c.Sync(key, w)
}

func (c *CommonController) syncDaemonSet(key string, obj *k8sappv1.DaemonSet) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	w, err := c.getWorkload(key, DaemonSetType)
	if err != nil || w == nil {
		return nil, err
	}
	return nil, c.Sync(key, w)
}

func (c *CommonController) syncStatefulSet(key string, obj *k8sappv1.StatefulSet) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	w, err := c.getWorkload(key, StatefulSetType)
	if err != nil || w == nil {
		return nil, err
	}
	return nil, c.Sync(key, w)
}

func (c *CommonController) syncJob(key string, obj *corebatchv1.Job) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	w, err := c.getWorkload(key, JobType)
	if err != nil || w == nil {
		return nil, err
	}
	return nil, c.Sync(key, w)
}

func (c *CommonController) syncCronJob(key string, obj *corebatchv1.CronJob) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	w, err := c.getWorkload(key, CronJobType)
	if err != nil || w == nil {
		return nil, err
	}
	return nil, c.Sync(key, w)
}

func (c CommonController) getWorkload(key string, objectType string) (*Workload, error) {
	splitted := strings.Split(key, "/")
	namespace := splitted[0]
	name := splitted[1]
	return c.GetByWorkloadID(getWorkloadID(objectType, namespace, name))
}

func (c CommonController) GetByWorkloadID(key string) (*Workload, error) {
	return c.getByWorkloadIDFromCacheOrAPI(key, false)
}

func (c CommonController) GetByWorkloadIDRetryAPIIfNotFound(key string) (*Workload, error) {
	return c.getByWorkloadIDFromCacheOrAPI(key, true)
}

func (c CommonController) getByWorkloadIDFromCacheOrAPI(key string, retry bool) (*Workload, error) {
	splitted := strings.Split(key, ":")
	if len(splitted) != 3 {
		return nil, fmt.Errorf("workload name [%s] is invalid", key)
	}
	workloadType := strings.ToLower(splitted[0])
	namespace := splitted[1]
	name := splitted[2]
	var workload *Workload
	switch workloadType {
	case ReplicationControllerType:
		o, err := c.ReplicationControllerLister.Get(namespace, name)
		if err != nil && apierrors.IsNotFound(err) && retry {
			o, err = c.ReplicationControllers.GetNamespaced(namespace, name, metav1.GetOptions{})
		}
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}
		labelSelector := &metav1.LabelSelector{
			MatchLabels: o.Spec.Selector,
		}
		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, labelSelector, o.Annotations, o.Spec.Template,
			o.OwnerReferences, o.Labels, o.Status.Replicas, o.Status.AvailableReplicas, convert.ToMapSlice(o.Status.Conditions))
	case ReplicaSetType:
		o, err := c.ReplicaSetLister.Get(namespace, name)
		if err != nil && apierrors.IsNotFound(err) && retry {
			o, err = c.ReplicaSets.GetNamespaced(namespace, name, metav1.GetOptions{})
		}
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template,
			o.OwnerReferences, o.Labels, o.Status.Replicas, o.Status.AvailableReplicas, convert.ToMapSlice(o.Status.Conditions))
	case DaemonSetType:
		o, err := c.DaemonSetLister.Get(namespace, name)
		if err != nil && apierrors.IsNotFound(err) && retry {
			o, err = c.DaemonSets.GetNamespaced(namespace, name, metav1.GetOptions{})
		}
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template,
			o.OwnerReferences, o.Labels, o.Status.DesiredNumberScheduled, o.Status.NumberAvailable, convert.ToMapSlice(o.Status.Conditions))
	case StatefulSetType:
		o, err := c.StatefulSetLister.Get(namespace, name)
		if err != nil && apierrors.IsNotFound(err) && retry {
			o, err = c.StatefulSets.GetNamespaced(namespace, name, metav1.GetOptions{})
		}
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, workloadType, AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template,
			o.OwnerReferences, o.Labels, o.Status.Replicas, o.Status.ReadyReplicas, convert.ToMapSlice(o.Status.Conditions))
	case JobType:
		o, err := c.JobLister.Get(namespace, name)
		if err != nil && apierrors.IsNotFound(err) && retry {
			o, err = c.Jobs.GetNamespaced(namespace, name, metav1.GetOptions{})
		}
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}
		var labelSelector *metav1.LabelSelector
		if o.Spec.Selector != nil {
			labelSelector = &metav1.LabelSelector{
				MatchLabels: o.Spec.Selector.MatchLabels,
			}
		}

		workload = getWorkload(namespace, name, workloadType, BatchVersion, o.UID, labelSelector, o.Annotations, &o.Spec.Template,
			o.OwnerReferences, o.Labels, 0, 0, convert.ToMapSlice(o.Status.Conditions))
	case CronJobType:
		o, err := c.CronJobLister.Get(namespace, name)
		if err != nil && apierrors.IsNotFound(err) && retry {
			o, err = c.CronJobs.GetNamespaced(namespace, name, metav1.GetOptions{})
		}
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}
		var labelSelector *metav1.LabelSelector
		if o.Spec.JobTemplate.Spec.Selector != nil {
			labelSelector = &metav1.LabelSelector{
				MatchLabels: o.Spec.JobTemplate.Spec.Selector.MatchLabels,
			}
		}

		workload = getWorkload(namespace, name, workloadType, BatchBetaVersion, o.UID, labelSelector, o.Annotations, &o.Spec.JobTemplate.Spec.Template,
			o.OwnerReferences, o.Labels, 0, 0, nil)
	default:
		o, err := c.DeploymentLister.Get(namespace, name)
		if err != nil && apierrors.IsNotFound(err) && retry {
			o, err = c.Deployments.GetNamespaced(namespace, name, metav1.GetOptions{})
		}
		if err != nil || o.DeletionTimestamp != nil {
			return nil, err
		}

		workload = getWorkload(namespace, name, DeploymentType, AppVersion, o.UID, o.Spec.Selector, o.Annotations, &o.Spec.Template,
			o.OwnerReferences, o.Labels, o.Status.Replicas, o.Status.AvailableReplicas, convert.ToMapSlice(o.Status.Conditions))
	}
	return workload, nil
}

func getWorkload(namespace string, name string, kind string, apiVersion string, UUID types.UID, selectorLabels *metav1.LabelSelector,
	annotations map[string]string, podTemplateSpec *corev1.PodTemplateSpec, ownerRefs []metav1.OwnerReference, labels map[string]string,
	replicas, availableReplicas int32, conditions []map[string]interface{}) *Workload {
	w := &Workload{
		Name:            name,
		Namespace:       namespace,
		SelectorLabels:  getSelectorLables(selectorLabels),
		UUID:            UUID,
		Annotations:     annotations,
		TemplateSpec:    podTemplateSpec,
		OwnerReferences: ownerRefs,
		Kind:            kind,
		APIVersion:      apiVersion,
		Labels:          labels,
		Key:             fmt.Sprintf("%s:%s:%s", kind, namespace, name),
		Status: &Status{
			Replicas:          replicas,
			AvailableReplicas: availableReplicas,
			Conditions:        conditions,
		},
	}
	return w
}

func (c CommonController) GetAllWorkloads(namespace string) ([]*Workload, error) {
	var workloads []*Workload

	// deployments
	ds, err := c.DeploymentLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, o := range ds {
		workload, err := c.GetByWorkloadID(getWorkloadID(DeploymentType, o.Namespace, o.Name))
		if err != nil || workload == nil {
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	// replication controllers
	rcs, err := c.ReplicationControllerLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, o := range rcs {
		workload, err := c.GetByWorkloadID(getWorkloadID(ReplicationControllerType, o.Namespace, o.Name))
		if err != nil || workload == nil {
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	// replica sets
	rss, err := c.ReplicaSetLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, o := range rss {
		workload, err := c.GetByWorkloadID(getWorkloadID(ReplicaSetType, o.Namespace, o.Name))
		if err != nil || workload == nil {
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	// daemon sets
	dss, err := c.DaemonSetLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, o := range dss {
		workload, err := c.GetByWorkloadID(getWorkloadID(DaemonSetType, o.Namespace, o.Name))
		if err != nil || workload == nil {
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	// stateful sets
	sts, err := c.StatefulSetLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, o := range sts {
		workload, err := c.GetByWorkloadID(getWorkloadID(StatefulSetType, o.Namespace, o.Name))
		if err != nil || workload == nil {
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	// jobs
	jobs, err := c.JobLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, o := range jobs {
		workload, err := c.GetByWorkloadID(getWorkloadID(JobType, o.Namespace, o.Name))
		if err != nil || workload == nil {
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	// cron jobs
	cronJobs, err := c.CronJobLister.List(namespace, labels.NewSelector())
	if err != nil {
		return workloads, err
	}

	for _, o := range cronJobs {
		workload, err := c.GetByWorkloadID(getWorkloadID(CronJobType, o.Namespace, o.Name))
		if err != nil || workload == nil {
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	return workloads, nil
}

func (c CommonController) GetWorkloadsMatchingLabels(namespace string, targetLabels map[string]string) ([]*Workload, error) {
	var workloads []*Workload
	allWorkloads, err := c.GetAllWorkloads(namespace)
	if err != nil {
		return workloads, err
	}

	for _, workload := range allWorkloads {
		workloadSelector := labels.SelectorFromSet(workload.SelectorLabels)
		if workloadSelector.Matches(labels.Set(targetLabels)) {
			workloads = append(workloads, workload)
		}
	}

	return workloads, nil
}

func (c CommonController) GetWorkloadsMatchingSelector(namespace string, selectorLabels map[string]string) ([]*Workload, error) {
	var workloads []*Workload
	allWorkloads, err := c.GetAllWorkloads(namespace)
	if err != nil {
		return workloads, err
	}

	selector := labels.SelectorFromSet(selectorLabels)
	for _, workload := range allWorkloads {
		if selector.Matches(labels.Set(workload.Labels)) {
			workloads = append(workloads, workload)
		}
	}

	return workloads, nil
}

func getSelectorLables(s *metav1.LabelSelector) map[string]string {
	if s == nil {
		return nil
	}
	selectorLabels := map[string]string{}
	for key, value := range s.MatchLabels {
		selectorLabels[key] = value
	}
	return selectorLabels
}

type Service struct {
	Type         corev1.ServiceType
	ClusterIP    string
	ServicePorts []corev1.ServicePort
	Name         string
}

type ContainerPort struct {
	Kind          string `json:"kind,omitempty"`
	SourcePort    int    `json:"sourcePort,omitempty"`
	DNSName       string `json:"dnsName,omitempty"`
	Name          string `json:"name,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	ContainerPort int32  `json:"containerPort,omitempty"`
}

func generateClusterIPServiceFromContainers(workload *Workload) *Service {
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
	clusterIP := ""
	// append default port as sky dns won't work w/o at least one port being set
	if len(servicePorts) == 0 {
		servicePort := corev1.ServicePort{
			Port:       42,
			TargetPort: intstr.Parse(strconv.FormatInt(42, 10)),
			Protocol:   corev1.Protocol(corev1.ProtocolTCP),
			Name:       "default",
		}
		clusterIP = "None"
		servicePorts = append(servicePorts, servicePort)
	}

	return &Service{
		Type:         ClusterIPServiceType,
		ClusterIP:    clusterIP,
		ServicePorts: servicePorts,
		Name:         workload.Name,
	}
}

func generateServicesFromPortsAnnotation(workload *Workload) ([]Service, error) {
	var services []Service
	val, ok := workload.TemplateSpec.Annotations[PortsAnnotation]
	if !ok {
		return services, nil
	}
	var portList [][]ContainerPort
	err := json.Unmarshal([]byte(val), &portList)
	if err != nil {
		return services, err
	}

	svcTypeToPort := map[corev1.ServiceType][]ContainerPort{}
	for _, l := range portList {
		for _, port := range l {
			if port.Kind == "HostPort" {
				continue
			}
			svcType := corev1.ServiceType(port.Kind)
			svcTypeToPort[svcType] = append(svcTypeToPort[svcType], port)
		}
	}

	for svcType, ports := range svcTypeToPort {
		servicePorts := map[string][]corev1.ServicePort{}
		for _, p := range ports {
			var nodePort int32
			var clusterIPPort int32
			if svcType == corev1.ServiceTypeNodePort {
				nodePort = int32(p.SourcePort)
				clusterIPPort = p.ContainerPort
			} else {
				if p.SourcePort == 0 {
					clusterIPPort = p.ContainerPort
				} else {
					clusterIPPort = int32(p.SourcePort)
				}
			}
			servicePort := corev1.ServicePort{
				Port:       clusterIPPort,
				TargetPort: intstr.Parse(strconv.FormatInt(int64(p.ContainerPort), 10)),
				NodePort:   nodePort,
				Protocol:   corev1.Protocol(p.Protocol),
				Name:       p.Name,
			}
			dnsName := p.DNSName
			if dnsName == "" {
				dnsName = workload.Name
			}
			servicePorts[dnsName] = append(servicePorts[dnsName], servicePort)
		}
		// append default port as sky dns won't work w/o at least one port being set
		if len(servicePorts) == 0 {
			servicePort := corev1.ServicePort{
				Port:       42,
				TargetPort: intstr.Parse(strconv.FormatInt(42, 10)),
				Protocol:   corev1.Protocol(corev1.ProtocolTCP),
				Name:       "default",
			}
			servicePorts[workload.Name] = append(servicePorts[workload.Name], servicePort)
		}

		for dnsName, servicePorts := range servicePorts {
			services = append(services, Service{
				Type:         svcType,
				ServicePorts: servicePorts,
				Name:         dnsName,
			})
		}

	}

	return services, nil
}

func getWorkloadID(objectType string, namespace string, name string) string {
	return fmt.Sprintf("%s:%s:%s", objectType, namespace, name)
}

func (c CommonController) UpdateWorkload(w *Workload, annotations map[string]string) error {
	// only annotations updates are supported
	switch w.Kind {
	case DeploymentType:
		o, err := c.DeploymentLister.Get(w.Namespace, w.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate := o.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			toUpdate.Annotations[key] = value
		}

		_, err = c.Deployments.Update(toUpdate)
		if err != nil {
			return err
		}
	case ReplicationControllerType:
		o, err := c.ReplicationControllerLister.Get(w.Namespace, w.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate := o.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			toUpdate.Annotations[key] = value
		}
		_, err = c.ReplicationControllers.Update(toUpdate)
		if err != nil {
			return err
		}
	case ReplicaSetType:
		o, err := c.ReplicaSetLister.Get(w.Namespace, w.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate := o.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			toUpdate.Annotations[key] = value
		}
		_, err = c.ReplicaSets.Update(toUpdate)
		if err != nil {
			return err
		}
	case DaemonSetType:
		o, err := c.DaemonSetLister.Get(w.Namespace, w.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate := o.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			toUpdate.Annotations[key] = value
		}
		_, err = c.DaemonSets.Update(toUpdate)
		if err != nil {
			return err
		}
	case StatefulSetType:
		o, err := c.StatefulSetLister.Get(w.Namespace, w.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate := o.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			toUpdate.Annotations[key] = value
		}
		_, err = c.StatefulSets.Update(toUpdate)
		if err != nil {
			return err
		}
	case JobType:
		o, err := c.JobLister.Get(w.Namespace, w.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate := o.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			toUpdate.Annotations[key] = value
		}
		_, err = c.Jobs.Update(toUpdate)
		if err != nil {
			return err
		}
	case CronJobType:
		o, err := c.CronJobLister.Get(w.Namespace, w.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate := o.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			toUpdate.Annotations[key] = value
		}
		_, err = c.CronJobs.Update(toUpdate)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c CommonController) EnqueueWorkload(w *Workload) {
	switch w.Kind {
	case DeploymentType:
		c.Deployments.Controller().Enqueue(w.Namespace, w.Name)
	case ReplicationControllerType:
		c.ReplicationControllers.Controller().Enqueue(w.Namespace, w.Name)
	case ReplicaSetType:
		c.ReplicaSets.Controller().Enqueue(w.Namespace, w.Name)
	case DaemonSetType:
		c.DaemonSets.Controller().Enqueue(w.Namespace, w.Name)
	case StatefulSetType:
		c.StatefulSets.Controller().Enqueue(w.Namespace, w.Name)
	case JobType:
		c.Jobs.Controller().Enqueue(w.Namespace, w.Name)
	case CronJobType:
		c.CronJobs.Controller().Enqueue(w.Namespace, w.Name)
	}
}

func (c CommonController) EnqueueAllWorkloads(namespace string) error {
	ws, err := c.GetAllWorkloads(namespace)
	if err != nil {
		return err
	}
	for _, w := range ws {
		c.EnqueueWorkload(w)
	}
	return nil
}

func (c CommonController) GetActualFromWorkload(w *Workload) (
	deploy *k8sappv1.Deployment,
	rc *corev1.ReplicationController,
	rs *k8sappv1.ReplicaSet,
	ds *k8sappv1.DaemonSet,
	ss *k8sappv1.StatefulSet,
	job *corebatchv1.Job,
	cj *corebatchv1.CronJob,
	err error,
) {
	switch w.Kind {
	case DeploymentType:
		deploy, err = c.DeploymentLister.Get(w.Namespace, w.Name)
	case ReplicationControllerType:
		rc, err = c.ReplicationControllerLister.Get(w.Namespace, w.Name)
	case ReplicaSetType:
		rs, err = c.ReplicaSetLister.Get(w.Namespace, w.Name)
	case DaemonSetType:
		ds, err = c.DaemonSetLister.Get(w.Namespace, w.Name)
	case StatefulSetType:
		ss, err = c.StatefulSetLister.Get(w.Namespace, w.Name)
	case JobType:
		job, err = c.JobLister.Get(w.Namespace, w.Name)
	case CronJobType:
		cj, err = c.CronJobLister.Get(w.Namespace, w.Name)
	}
	return
}
