package workload

import (
	"context"

	"strings"

	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/util"
	"github.com/rancher/types/config"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
)

type Controller struct {
	workloadLister util.WorkloadLister
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &Controller{
		workloadLister: util.NewWorkloadLister(workload),
	}
	workload.Apps.Deployments("").AddHandler(getName(), c.syncDeployments)
	workload.Core.ReplicationControllers("").AddHandler(getName(), c.syncReplicationControllers)
	workload.Apps.ReplicaSets("").AddHandler(getName(), c.syncReplicaSet)
	workload.Apps.DaemonSets("").AddHandler(getName(), c.syncDaemonSet)
	workload.Apps.StatefulSets("").AddHandler(getName(), c.syncStatefulSet)
	workload.BatchV1.Jobs("").AddHandler(getName(), c.syncJob)
	workload.BatchV1Beta1.CronJobs("").AddHandler(getName(), c.syncCronJob)
}

func getName() string {
	return "headlessServiceGenerationController"
}

func (c *Controller) syncDeployments(key string, obj *v1beta2.Deployment) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	return c.createService(key, "deployment")
}

func (c *Controller) syncReplicationControllers(key string, obj *v1.ReplicationController) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	return c.createService(key, "replicationcontroller")
}

func (c *Controller) syncReplicaSet(key string, obj *v1beta2.ReplicaSet) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	return c.createService(key, "replicaset")
}

func (c *Controller) syncDaemonSet(key string, obj *v1beta2.DaemonSet) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	return c.createService(key, "daemonset")
}

func (c *Controller) syncStatefulSet(key string, obj *v1beta2.StatefulSet) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	return c.createService(key, "statefulset")
}

func (c *Controller) syncJob(key string, obj *batchv1.Job) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	return c.createService(key, "job")
}

func (c *Controller) syncCronJob(key string, obj *batchv1beta1.CronJob) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	return c.createService(key, "cronjob")
}

func (c *Controller) createService(key string, objectType string) error {
	splitted := strings.Split(key, "/")
	namespace := splitted[0]
	name := splitted[1]

	workload, err := c.workloadLister.GetByName(fmt.Sprintf("%s:%s:%s", objectType, namespace, name))
	if err != nil {
		return err
	}
	return c.workloadLister.CreateServiceForWorkload(workload)
}
