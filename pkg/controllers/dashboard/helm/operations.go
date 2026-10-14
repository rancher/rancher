package helm

import (
	"context"
	"fmt"
	"reflect"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kstatus"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
)

const (
	podIndex = "byPod"
)

type operationHandler struct {
	ctx             context.Context
	pods            corecontrollers.PodCache
	k8s             kubernetes.Interface
	operationsCache catalogcontrollers.OperationCache
	apps            catalogcontrollers.AppController
}

func RegisterOperations(ctx context.Context,
	k8s kubernetes.Interface,
	pods corecontrollers.PodController,
	operations catalogcontrollers.OperationController,
	apps catalogcontrollers.AppController) {

	o := operationHandler{
		ctx:             ctx,
		k8s:             k8s,
		pods:            pods.Cache(),
		operationsCache: operations.Cache(),
		apps:            apps,
	}

	operations.Cache().AddIndexer(podIndex, indexOperationsByPod)
	relatedresource.Watch(ctx, "helm-operation", o.findOperationsFromPod, operations, pods)
	catalogcontrollers.RegisterOperationStatusHandler(ctx, operations, "", "helm-operation", o.onOperationChange)
}

func indexOperationsByPod(obj *catalog.Operation) ([]string, error) {
	return []string{
		obj.Status.PodNamespace + "/" + obj.Status.PodName,
	}, nil
}

func (o *operationHandler) findOperationsFromPod(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	ops, err := o.operationsCache.GetByIndex(podIndex, namespace+"/"+name)
	if err != nil {
		return nil, err
	}
	var result []relatedresource.Key
	for _, op := range ops {
		result = append(result, relatedresource.NewKey(op.Namespace, op.Name))
	}
	return result, nil
}

func (o *operationHandler) onOperationChange(operation *catalog.Operation, status catalog.OperationStatus) (catalog.OperationStatus, error) {
	if status.PodName == "" || status.PodNamespace == "" {
		return status, nil
	}

	pod, err := o.pods.Get(status.PodNamespace, status.PodName)
	if apierrors.IsNotFound(err) {
		kstatus.SetActive(&status)
		return status, nil
	}

	for _, container := range pod.Status.ContainerStatuses {
		if container.Name != "helm" {
			continue
		}
		status.ObservedGeneration = operation.Generation
		if container.State.Running != nil {
			status.PodCreated = true
			kstatus.SetTransitioning(&status, "running operation")
		} else if container.State.Terminated != nil {
			status.PodCreated = true
			if container.State.Terminated.ExitCode == 0 {
				kstatus.SetActive(&status)
				if operation.Status.Action != "uninstall" {
					retry.OnError(wait.Backoff{Steps: 3, Duration: 1 * time.Second, Factor: 2, Jitter: 0.1},
						func(err error) bool {
							return apierrors.IsConflict(err) || apierrors.IsNotFound(err)
						},
						func() error {
							app, err := o.apps.Get(operation.Status.Namespace, operation.Status.Release, metav1.GetOptions{})
							if err != nil {
								return err
							}
							app.Status.AutomaticCPTolerations = operation.Status.AutomaticCPTolerations
							if !reflect.DeepEqual(app.Status.Tolerations, operation.Status.Tolerations) {
								app.Status.Tolerations = operation.Status.Tolerations
							}
							_, err = o.apps.UpdateStatus(app)
							if err != nil {
								logrus.Errorf("failed to update app.status.tolerations: %v", err)
								return err
							}
							return nil
						},
					)
				}
			} else {
				kstatus.SetError(&status,
					fmt.Sprintf("%s exit code: %d",
						container.State.Terminated.Message,
						container.State.Terminated.ExitCode))
			}
			if err := o.cleanup(pod); err != nil {
				return status, err
			}
		} else if container.State.Waiting != nil {
			kstatus.SetTransitioning(&status, "waiting to run operation")
		} else {
			kstatus.SetTransitioning(&status, "unknown state operation")
		}
	}

	return status, nil
}

func (o *operationHandler) cleanup(pod *corev1.Pod) error {
	running := false
	success := false
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == "proxy" && container.State.Terminated == nil {
			running = true
		} else if container.Name == "helm" && container.State.Terminated != nil && container.State.Terminated.ExitCode == 0 {
			success = true
		}
	}

	if !running || !success {
		return nil
	}

	result := o.k8s.CoreV1().RESTClient().
		Get().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		SetHeader("Upgrade", "websocket").
		SetHeader("Sec-Websocket-Key", "websocket").
		SetHeader("Sec-Websocket-Version", "13").
		SetHeader("Connection", "Upgrade").
		VersionedParams(&corev1.PodExecOptions{
			Stdout:    true,
			Stderr:    true,
			Container: "proxy",
			Command:   []string{"pkill", "-x", "kubectl"},
		}, scheme.ParameterCodec).Do(o.ctx)
	return result.Error()
}
