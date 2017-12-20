package workload

import (
	"context"

	"github.com/rancher/norman/offspring"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	trueValue        = true
	workloadSchema   = schema.Schemas.Schema(&schema.Version, client.WorkloadType)
	deploymentSchema = schema.Schemas.Schema(&schema.Version, client.DeploymentType)
)

func Register(ctx context.Context, workload *config.WorkloadContext) {
	c := &Controller{}

	r := offspring.NewReconciliation(ctx,
		c.Generate,
		workload.Project.Workloads("").Controller().Enqueue,
		workload.Project.Workloads("").ObjectClient(),
		offspring.ChildWatcher{
			ObjectClient: workload.Apps.Deployments("").ObjectClient(),
			Informer:     workload.Apps.Deployments("").Controller().Informer(),
		})

	workload.Project.Workloads("").AddSyncHandler(func(key string, obj *v3.Workload) error {
		_, err := r.Changed(key, obj)
		return err
	})
}

type Controller struct {
}

func (c *Controller) Generate(obj runtime.Object) (offspring.ObjectSet, error) {
	workload := obj.(*v3.Workload)
	result := offspring.ObjectSet{
		Complete: true,
	}

	deployment, err := convertWorkloadToDeployment(workload)
	if err != nil {
		return result, err
	}

	result.Children = append(result.Children, deployment)

	return result, nil
}

func convertWorkloadToDeployment(workload *v3.Workload) (*v1beta2.Deployment, error) {
	deployment := &v1beta2.Deployment{}
	if err := convertTypes(workload, workloadSchema, deploymentSchema, deployment); err != nil {
		return deployment, err
	}

	deployment.Kind = "Deployment"
	deployment.APIVersion = "apps/v1beta2"
	deployment.Spec.Selector = getSelector(workload.Name)
	deployment.Spec.Template.Labels = getLabels(deployment.Spec.Template.Labels, workload.Name)

	return deployment, nil
}

func getLabels(labels map[string]string, name string) map[string]string {
	if labels == nil {
		labels = map[string]string{}
	}
	labels["workload.cattle.io/name"] = name
	return labels
}

func getSelector(name string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"workload.cattle.io/name": name,
		},
	}
}

func convertTypes(workload *v3.Workload, fromSchema *types.Schema, toSchema *types.Schema, target interface{}) error {
	data, err := convert.EncodeToMap(workload)
	if err != nil {
		return err
	}
	fromSchema.Mapper.FromInternal(data)
	toSchema.Mapper.ToInternal(data)
	return convert.ToObj(data, target)
}
