package machinedrain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/controller"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/drain"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type handler struct {
	ctx          context.Context
	machineCache capicontrollers.MachineCache
	secrets      corecontrollers.SecretClient
	secretCache  corecontrollers.SecretCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		ctx:          ctx,
		machineCache: clients.CAPI.Machine().Cache(),
		secrets:      clients.Core.Secret(),
		secretCache:  clients.Core.Secret().Cache(),
	}

	// It would be a bad idea to have this handler run for every secret. Therefore, a new cache factory is created just for secrets with the machine plan type.
	cacheFactory := cache.NewSharedCachedFactory(clients.ControllerFactory.SharedCacheFactory().SharedClientFactory(), &cache.SharedCacheFactoryOptions{
		DefaultTweakList: func(options *metav1.ListOptions) {
			options.TypeMeta = metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			}
			options.FieldSelector = fmt.Sprintf("type=%s", rke2.SecretTypeMachinePlan)
		},
	})

	controllerFactory := controller.NewSharedControllerFactory(cacheFactory, &controller.SharedControllerFactoryOptions{
		DefaultRateLimiter: workqueue.NewItemExponentialFailureRateLimiter(1*time.Minute, 5*time.Minute),
		DefaultWorkers:     1,
	})

	corecontrollers.New(controllerFactory).Secret().OnChange(ctx, "machine-drain", h.OnChange)
	if err := controllerFactory.Start(ctx, 1); err != nil {
		panic(err)
	}
}

func (h *handler) OnChange(_ string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.DeletionTimestamp != nil || secret.Labels[rke2.MachineNameLabel] == "" {
		return secret, nil
	}

	machine, err := h.machineCache.Get(secret.Namespace, secret.Labels[rke2.MachineNameLabel])
	if err != nil {
		return secret, err
	}

	drain := secret.Annotations[rke2.DrainAnnotation]
	if drain != "" && secret.Annotations[rke2.DrainDoneAnnotation] != drain {
		return h.drain(secret, machine, drain)
	}

	// Only check that it's non-blank.  There is no correlation between the drain and unDrain options, meaning
	// that the option values do not need to match.  For drain we track the status by doing the drain annotation
	// and then adding a drain-done annotation with the same value when it's done.  Uncordon is different in that
	// we want the final state to have no annotations.  So when UnCordonAnnotation is set we run and then when
	// it's done we delete it.  So there is no knowledge of what the value should be except that it's set.
	if secret.Annotations[rke2.UnCordonAnnotation] != "" {
		return h.unDrain(secret, machine, drain)
	}

	return secret, nil
}

func (h *handler) k8sClient(machine *capi.Machine) (kubernetes.Interface, error) {
	secret, err := h.secretCache.Get(machine.Namespace, name.SafeConcatName(machine.Spec.ClusterName, "kubeconfig"))
	if err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restConfig)
}

func (h *handler) unDrain(secret *corev1.Secret, machine *capi.Machine, drainData string) (*corev1.Secret, error) {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		logrus.Debugf("unable to drain machine %s as there is no noderef", machine.Name)
		return secret, nil
	}

	var drainOpts rkev1.DrainOptions
	if err := json.Unmarshal([]byte(drainData), &drainOpts); err != nil {
		return secret, err
	}

	if len(drainOpts.PostDrainHooks) > 0 {
		if secret.Annotations[rke2.PostDrainAnnotation] != drainData {
			secret = secret.DeepCopy()
			secret.Annotations[rke2.PostDrainAnnotation] = drainData
			return h.secrets.Update(secret)
		}
		for _, hook := range drainOpts.PostDrainHooks {
			if hook.Annotation != "" && secret.Annotations[hook.Annotation] != drainData {
				return secret, nil
			}
		}
	}

	helper, node, err := h.getHelper(machine, drainOpts)
	if err != nil {
		return nil, err
	}

	if err := drain.RunCordonOrUncordon(helper, node, false); err != nil {
		return nil, err
	}

	// Drain/Undrain operations are done so clear all annotations involved
	secret = secret.DeepCopy()
	delete(secret.Annotations, rke2.PreDrainAnnotation)
	delete(secret.Annotations, rke2.PostDrainAnnotation)
	delete(secret.Annotations, rke2.DrainAnnotation)
	delete(secret.Annotations, rke2.DrainDoneAnnotation)
	delete(secret.Annotations, rke2.UnCordonAnnotation)
	for _, hook := range drainOpts.PreDrainHooks {
		delete(secret.Annotations, hook.Annotation)
	}
	for _, hook := range drainOpts.PostDrainHooks {
		delete(secret.Annotations, hook.Annotation)
	}
	return h.secrets.Update(secret)
}

func (h *handler) drain(secret *corev1.Secret, machine *capi.Machine, drainData string) (*corev1.Secret, error) {
	drainOpts := &rkev1.DrainOptions{}
	if err := json.Unmarshal([]byte(drainData), drainOpts); err != nil {
		return nil, err
	}

	if err := h.cordon(machine, drainOpts); err != nil {
		return secret, err
	}

	if len(drainOpts.PreDrainHooks) > 0 {
		if secret.Annotations[rke2.PreDrainAnnotation] != drainData {
			secret = secret.DeepCopy()
			secret.Annotations[rke2.PreDrainAnnotation] = drainData
			return h.secrets.Update(secret)
		}
		for _, hook := range drainOpts.PreDrainHooks {
			if hook.Annotation != "" && secret.Annotations[hook.Annotation] != drainData {
				return secret, nil
			}
		}
	}

	if drainOpts.Enabled {
		if err := h.performDrain(machine, drainOpts); err != nil {
			return nil, err
		}
	}

	secret = secret.DeepCopy()
	secret.Annotations[rke2.DrainDoneAnnotation] = drainData
	return h.secrets.Update(secret)
}

func (h *handler) cordon(machine *capi.Machine, drainOpts *rkev1.DrainOptions) error {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		return nil
	}

	helper, node, err := h.getHelper(machine, *drainOpts)
	if err != nil {
		return err
	}

	return drain.RunCordonOrUncordon(helper, node, true)
}

func (h *handler) getHelper(machine *capi.Machine, drainOpts rkev1.DrainOptions) (*drain.Helper, *corev1.Node, error) {
	k8s, err := h.k8sClient(machine)
	if err != nil {
		return nil, nil, err
	}

	timeout := drainOpts.Timeout
	if timeout == 0 {
		timeout = 600
	}

	helper := &drain.Helper{
		Ctx:                             h.ctx,
		Client:                          k8s,
		Force:                           drainOpts.Force,
		GracePeriodSeconds:              drainOpts.GracePeriod,
		IgnoreAllDaemonSets:             drainOpts.IgnoreDaemonSets == nil || *drainOpts.IgnoreDaemonSets,
		IgnoreErrors:                    drainOpts.IgnoreErrors,
		Timeout:                         time.Duration(timeout) * time.Second,
		DeleteEmptyDirData:              drainOpts.DeleteEmptyDirData,
		DisableEviction:                 drainOpts.DisableEviction,
		SkipWaitForDeleteTimeoutSeconds: drainOpts.SkipWaitForDeleteTimeoutSeconds,
		Out:                             os.Stdout,
		ErrOut:                          os.Stderr,
	}

	node, err := k8s.CoreV1().Nodes().Get(h.ctx, machine.Status.NodeRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	return helper, node, err
}

func (h *handler) performDrain(machine *capi.Machine, drainOpts *rkev1.DrainOptions) error {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		return nil
	}

	helper, node, err := h.getHelper(machine, *drainOpts)
	if err != nil {
		return err
	}

	return drain.RunNodeDrain(helper, node.Name)
}
