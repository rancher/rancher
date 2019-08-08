package windows

import (
	"fmt"
	"strings"

	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/taints"
	typesv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	v1beta2fakes "github.com/rancher/types/apis/apps/v1beta2/fakes"
	typesbatchv1 "github.com/rancher/types/apis/batch/v1"
	batchv1fakes "github.com/rancher/types/apis/batch/v1/fakes"
	typesbatchv1beta1 "github.com/rancher/types/apis/batch/v1beta1"
	batchv1beta1fakes "github.com/rancher/types/apis/batch/v1beta1/fakes"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	v1fakes "github.com/rancher/types/apis/core/v1/fakes"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"
)

func newFakeCommonController(cases []workloadTestCase, wrs []workloadRefenece) util.CommonController {
	items := getItemsFromWorkload(wrs)
	c := util.CommonController{
		DeploymentLister:            getFakeDeploymentLister(items),
		ReplicationControllerLister: getFakeReplicaControllerLister(items),
		ReplicaSetLister:            getFakeReplicaSetLister(items),
		DaemonSetLister:             getFakeDaemonSetLister(items),
		StatefulSetLister:           getFakeStatefulSetLister(items),
		JobLister:                   getFakeJobLister(items),
		CronJobLister:               getFakeCronJobLister(items),
		Deployments:                 newDeploymentInterface(cases, wrs),
		ReplicationControllers:      newReplicationControllerInterface(cases, wrs),
		ReplicaSets:                 newReplicaSetInterface(cases, wrs),
		DaemonSets:                  newDaemonSetInterface(cases, wrs),
		StatefulSets:                newStatefulSetInterface(cases, wrs),
		Jobs:                        newJobInterface(cases, wrs),
		CronJobs:                    newCronJobInterface(cases, wrs),
	}
	return c
}

func getFakeDeploymentLister(items []runtime.Object) typesv1beta2.DeploymentLister {
	return &v1beta2fakes.DeploymentListerMock{
		GetFunc: func(namespace string, name string) (*v1beta2.Deployment, error) {
			for _, item := range items {
				v, ok := item.(*v1beta2.Deployment)
				if !ok {
					continue
				}
				if v.Namespace == namespace && v.Name == name {
					return v, nil
				}
			}
			return nil, apierrors.NewNotFound(getGroupResource(typesv1beta2.DeploymentGroupVersionKind), getKey(namespace, name))
		},
		ListFunc: func(namespace string, selector labels.Selector) ([]*v1beta2.Deployment, error) {
			var rtn []*v1beta2.Deployment
			for _, item := range items {
				v, ok := item.(*v1beta2.Deployment)
				if ok && v.Namespace == namespace && selector.Matches(labels.Set(v.Labels)) {
					rtn = append(rtn, v)
				}
			}
			return rtn, nil
		},
	}
}

func getFakeReplicaSetLister(items []runtime.Object) typesv1beta2.ReplicaSetLister {
	return &v1beta2fakes.ReplicaSetListerMock{
		GetFunc: func(namespace string, name string) (*v1beta2.ReplicaSet, error) {
			for _, item := range items {
				v, ok := item.(*v1beta2.ReplicaSet)
				if !ok {
					continue
				}
				if v.Namespace == namespace && v.Name == name {
					return v, nil
				}
			}
			return nil, apierrors.NewNotFound(getGroupResource(typesv1beta2.ReplicaSetGroupVersionKind), getKey(namespace, name))
		},
		ListFunc: func(namespace string, selector labels.Selector) ([]*v1beta2.ReplicaSet, error) {
			println("get in list func")
			var rtn []*v1beta2.ReplicaSet
			for _, item := range items {
				v, ok := item.(*v1beta2.ReplicaSet)
				if ok && v.Namespace == namespace && selector.Matches(labels.Set(v.Labels)) {
					rtn = append(rtn, v)
				}
			}
			return rtn, nil
		},
	}
}

func getFakeReplicaControllerLister(items []runtime.Object) typescorev1.ReplicationControllerLister {
	return &v1fakes.ReplicationControllerListerMock{
		GetFunc: func(namespace string, name string) (*corev1.ReplicationController, error) {
			for _, item := range items {
				v, ok := item.(*corev1.ReplicationController)
				if !ok {
					continue
				}
				if v.Namespace == namespace && v.Name == name {
					return v, nil
				}
			}
			return nil, apierrors.NewNotFound(getGroupResource(typescorev1.ReplicationControllerGroupVersionKind), getKey(namespace, name))
		},
		ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.ReplicationController, error) {
			var rtn []*corev1.ReplicationController
			for _, item := range items {
				v, ok := item.(*corev1.ReplicationController)
				if ok && v.Namespace == namespace && selector.Matches(labels.Set(v.Labels)) {
					rtn = append(rtn, v)
				}
			}
			return rtn, nil
		},
	}
}

func getFakeDaemonSetLister(items []runtime.Object) typesv1beta2.DaemonSetLister {
	return &v1beta2fakes.DaemonSetListerMock{
		GetFunc: func(namespace string, name string) (*v1beta2.DaemonSet, error) {
			for _, item := range items {
				v, ok := item.(*v1beta2.DaemonSet)
				if !ok {
					continue
				}
				if v.Namespace == namespace && v.Name == name {
					return v, nil
				}
			}
			return nil, apierrors.NewNotFound(getGroupResource(typesv1beta2.DaemonSetGroupVersionKind), getKey(namespace, name))
		},
		ListFunc: func(namespace string, selector labels.Selector) ([]*v1beta2.DaemonSet, error) {
			var rtn []*v1beta2.DaemonSet
			for _, item := range items {
				v, ok := item.(*v1beta2.DaemonSet)
				if ok && v.Namespace == namespace && selector.Matches(labels.Set(v.Labels)) {
					rtn = append(rtn, v)
				}
			}
			return rtn, nil
		},
	}
}

func getFakeStatefulSetLister(items []runtime.Object) typesv1beta2.StatefulSetLister {
	return &v1beta2fakes.StatefulSetListerMock{
		GetFunc: func(namespace string, name string) (*v1beta2.StatefulSet, error) {
			for _, item := range items {
				v, ok := item.(*v1beta2.StatefulSet)
				if !ok {
					continue
				}
				if v.Namespace == namespace && v.Name == name {
					return v, nil
				}
			}
			return nil, apierrors.NewNotFound(getGroupResource(typesv1beta2.StatefulSetGroupVersionKind), getKey(namespace, name))
		},
		ListFunc: func(namespace string, selector labels.Selector) ([]*v1beta2.StatefulSet, error) {
			var rtn []*v1beta2.StatefulSet
			for _, item := range items {
				v, ok := item.(*v1beta2.StatefulSet)
				if ok && v.Namespace == namespace && selector.Matches(labels.Set(v.Labels)) {
					rtn = append(rtn, v)
				}
			}
			return rtn, nil
		},
	}
}

func getFakeJobLister(items []runtime.Object) typesbatchv1.JobLister {
	return &batchv1fakes.JobListerMock{
		GetFunc: func(namespace string, name string) (*batchv1.Job, error) {
			for _, item := range items {
				v, ok := item.(*batchv1.Job)
				if !ok {
					continue
				}
				if v.Namespace == namespace && v.Name == name {
					return v, nil
				}
			}
			return nil, apierrors.NewNotFound(getGroupResource(typesbatchv1.JobGroupVersionKind), getKey(namespace, name))
		},
		ListFunc: func(namespace string, selector labels.Selector) ([]*batchv1.Job, error) {
			var rtn []*batchv1.Job
			for _, item := range items {
				v, ok := item.(*batchv1.Job)
				if ok && v.Namespace == namespace && selector.Matches(labels.Set(v.Labels)) {
					rtn = append(rtn, v)
				}
			}
			return rtn, nil
		},
	}
}

func getFakeCronJobLister(items []runtime.Object) typesbatchv1beta1.CronJobLister {
	return &batchv1beta1fakes.CronJobListerMock{
		GetFunc: func(namespace string, name string) (*batchv1beta1.CronJob, error) {
			for _, item := range items {
				v, ok := item.(*batchv1beta1.CronJob)
				if !ok {
					continue
				}
				if v.Namespace == namespace && v.Name == name {
					return v, nil
				}
			}
			return nil, apierrors.NewNotFound(getGroupResource(typesbatchv1beta1.CronJobGroupVersionKind), getKey(namespace, name))
		},
		ListFunc: func(namespace string, selector labels.Selector) ([]*batchv1beta1.CronJob, error) {
			var rtn []*batchv1beta1.CronJob
			for _, item := range items {
				v, ok := item.(*batchv1beta1.CronJob)
				if ok && v.Namespace == namespace && selector.Matches(labels.Set(v.Labels)) {
					rtn = append(rtn, v)
				}
			}
			return rtn, nil
		},
	}
}

func getKey(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}

func getResourceName(groupVersion schema.GroupVersionKind) string {
	return strings.ToLower(groupVersion.Kind[0:1]) + groupVersion.Kind[1:]
}

func getGroupResource(groupVersion schema.GroupVersionKind) schema.GroupResource {
	return schema.GroupResource{
		Group:    groupVersion.Group,
		Resource: getResourceName(groupVersion),
	}
}

func newDeploymentInterface(cases []workloadTestCase, wrs []workloadRefenece) typesv1beta2.DeploymentInterface {
	return &v1beta2fakes.DeploymentInterfaceMock{
		UpdateFunc: func(in1 *v1beta2.Deployment) (*v1beta2.Deployment, error) {
			for index, wr := range wrs {
				if wr.deploy == nil ||
					in1.Name != wr.deploy.Name ||
					in1.Namespace != wr.deploy.Namespace {
					continue
				}
				return in1, validateUpdate(cases[index], in1)
			}
			return in1, fmt.Errorf("workload %s/%s not found in use cases", in1.Namespace, in1.Name)
		},
	}
}

func newReplicationControllerInterface(cases []workloadTestCase, wrs []workloadRefenece) typescorev1.ReplicationControllerInterface {
	return &v1fakes.ReplicationControllerInterfaceMock{
		UpdateFunc: func(in1 *corev1.ReplicationController) (*corev1.ReplicationController, error) {
			for index, wr := range wrs {
				if wr.rc == nil ||
					in1.Name != wr.rc.Name ||
					in1.Namespace != wr.rc.Namespace {
					continue
				}
				return in1, validateUpdate(cases[index], in1)
			}
			return in1, fmt.Errorf("workload %s/%s not found in use cases", in1.Namespace, in1.Name)
		},
	}
}

func newReplicaSetInterface(cases []workloadTestCase, wrs []workloadRefenece) typesv1beta2.ReplicaSetInterface {
	return &v1beta2fakes.ReplicaSetInterfaceMock{
		UpdateFunc: func(in1 *v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error) {
			for index, wr := range wrs {
				if wr.rs == nil ||
					in1.Name != wr.rs.Name ||
					in1.Namespace != wr.rs.Namespace {
					continue
				}
				return in1, validateUpdate(cases[index], in1)
			}
			return in1, fmt.Errorf("workload %s/%s not found in use cases", in1.Namespace, in1.Name)
		},
	}
}

func newStatefulSetInterface(cases []workloadTestCase, wrs []workloadRefenece) typesv1beta2.StatefulSetInterface {
	return &v1beta2fakes.StatefulSetInterfaceMock{
		UpdateFunc: func(in1 *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error) {
			for index, wr := range wrs {
				if wr.ss == nil ||
					in1.Name != wr.ss.Name ||
					in1.Namespace != wr.ss.Namespace {
					continue
				}
				return in1, validateUpdate(cases[index], in1)
			}
			return in1, fmt.Errorf("workload %s/%s not found in use cases", in1.Namespace, in1.Name)
		},
	}
}

func newDaemonSetInterface(cases []workloadTestCase, wrs []workloadRefenece) typesv1beta2.DaemonSetInterface {
	return &v1beta2fakes.DaemonSetInterfaceMock{
		UpdateFunc: func(in1 *v1beta2.DaemonSet) (*v1beta2.DaemonSet, error) {
			for index, wr := range wrs {
				if wr.ds == nil ||
					in1.Name != wr.ds.Name ||
					in1.Namespace != wr.ds.Namespace {
					continue
				}
				return in1, validateUpdate(cases[index], in1)
			}
			return in1, fmt.Errorf("workload %s/%s not found in use cases", in1.Namespace, in1.Name)
		},
	}
}

func newJobInterface(cases []workloadTestCase, wrs []workloadRefenece) typesbatchv1.JobInterface {
	return &batchv1fakes.JobInterfaceMock{
		UpdateFunc: func(in1 *batchv1.Job) (*batchv1.Job, error) {
			for index, wr := range wrs {
				if wr.job == nil ||
					in1.Name != wr.job.Name ||
					in1.Namespace != wr.job.Namespace {
					continue
				}
				return in1, validateUpdate(cases[index], in1)
			}
			return in1, fmt.Errorf("workload %s/%s not found in use cases", in1.Namespace, in1.Name)
		},
	}
}

func newCronJobInterface(cases []workloadTestCase, wrs []workloadRefenece) typesbatchv1beta1.CronJobInterface {
	return &batchv1beta1fakes.CronJobInterfaceMock{
		UpdateFunc: func(in1 *batchv1beta1.CronJob) (*batchv1beta1.CronJob, error) {
			for index, wr := range wrs {
				if wr.cj == nil ||
					in1.Name != wr.cj.Name ||
					in1.Namespace != wr.cj.Namespace {
					continue
				}
				return in1, validateUpdate(cases[index], in1)
			}
			return in1, fmt.Errorf("workload %s/%s not found in use cases", in1.Namespace, in1.Name)
		},
	}
}

func getPodSpec(workload interface{}) corev1.PodSpec {
	switch workload.(type) {
	case *v1beta2.Deployment:
		deploy := workload.(*v1beta2.Deployment)
		return deploy.Spec.Template.Spec
	case *corev1.ReplicationController:
		rc := workload.(*corev1.ReplicationController)
		return rc.Spec.Template.Spec
	case *v1beta2.ReplicaSet:
		rs := workload.(*v1beta2.ReplicaSet)
		return rs.Spec.Template.Spec
	case *v1beta2.StatefulSet:
		ss := workload.(*v1beta2.StatefulSet)
		return ss.Spec.Template.Spec
	case *v1beta2.DaemonSet:
		ds := workload.(*v1beta2.DaemonSet)
		return ds.Spec.Template.Spec
	case *batchv1.Job:
		job := workload.(*batchv1.Job)
		return job.Spec.Template.Spec
	case *batchv1beta1.CronJob:
		cj := workload.(*batchv1beta1.CronJob)
		return cj.Spec.JobTemplate.Spec.Template.Spec
	}
	return corev1.PodSpec{}
}

func validateUpdate(c workloadTestCase, in1 runtime.Object) error {
	if !c.shouldUpdate {
		return fmt.Errorf("workload %s should be get in update function", c.reference.workload.Key)
	}
	podSpec := getPodSpec(in1)
	if !helper.TolerationsTolerateTaint(podSpec.Tolerations, &taints.NodeTaint) {
		return fmt.Errorf("workload %s should have toleration for taint %+v", c.reference.workload.Key, taints.NodeTaint)
	}
	return nil
}
