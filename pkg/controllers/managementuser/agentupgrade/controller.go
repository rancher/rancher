package agentupgrade

import (
	"context"
	"regexp"

	v1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	"github.com/rancher/rancher/pkg/types/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	namespace = "cattle-system"
)

var (
	names = map[string]bool{
		"cattle-cluster-agent": true,
		"cattle-node-agent":    true,
	}
	imageRegexp = regexp.MustCompile("v2\\.[0-4]\\.")
)

type handler struct {
	deployments     v1.DeploymentInterface
	daemonsetClient v1.DaemonSetInterface
}

func Register(ctx context.Context, context *config.UserOnlyContext) error {
	h := &handler{
		deployments:     context.Apps.Deployments(""),
		daemonsetClient: context.Apps.DaemonSets(""),
	}

	context.Apps.Deployments("").Controller().AddHandler(ctx, "agent-upgrade", h.OnDeploymentChange)
	context.Apps.DaemonSets("").Controller().AddHandler(ctx, "agent-upgrade", h.OnDaemonSetChange)
	return nil
}

func (h *handler) OnDeploymentChange(key string, deploy *appsv1.Deployment) (runtime.Object, error) {
	if deploy == nil || !shouldDelete(&deploy.ObjectMeta, &deploy.Spec.Template) {
		return deploy, nil
	}
	return deploy, h.deployments.DeleteNamespaced(deploy.Namespace, deploy.Name, nil)
}

func (h *handler) OnDaemonSetChange(key string, ds *appsv1.DaemonSet) (runtime.Object, error) {
	if ds == nil || !shouldDelete(&ds.ObjectMeta, &ds.Spec.Template) {
		return ds, nil
	}
	return ds, h.daemonsetClient.DeleteNamespaced(ds.Namespace, ds.Name, nil)
}

func shouldDelete(meta *metav1.ObjectMeta, pod *corev1.PodTemplateSpec) bool {
	if meta.Namespace != namespace {
		return false
	}

	if !names[meta.Name] {
		return false
	}

	if len(pod.Spec.Containers) == 0 {
		return false
	}

	return imageRegexp.MatchString(pod.Spec.Containers[0].Image)
}
