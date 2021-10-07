package machinedrain

import (
	"context"
	"encoding/json"
	"os"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/drain"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type handler struct {
	ctx      context.Context
	machines capicontrollers.MachineClient
	secrets  corecontrollers.SecretCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		ctx:      ctx,
		machines: clients.CAPI.Machine(),
		secrets:  clients.Core.Secret().Cache(),
	}
	clients.CAPI.Machine().OnChange(ctx, "machine-drain", h.OnChange)
}

func (h *handler) OnChange(key string, machine *capi.Machine) (*capi.Machine, error) {
	if machine == nil {
		return nil, nil
	}

	drain := machine.Annotations[planner.DrainAnnotation]
	if drain != "" && machine.Annotations[planner.DrainDoneAnnotation] != drain {
		return h.drain(machine, drain)
	}

	// Only check that it's non-blank.  There is no correlation between the drain and undrain options, meaning
	// that the option values do not need to match.  For drain we track the status by doing the drain annotation
	// and then adding a drain-done annotation with the same value when it's done.  Uncordon is different in that
	// we want the final state to have no annotations.  So when UnCordonAnnotation is set we run and then when
	// it's done we delete it.  So there is no knowledge of what the value should be except that it's set.
	if machine.Annotations[planner.UnCordonAnnotation] != "" {
		return h.undrain(machine, drain)
	}

	return machine, nil
}

func (h *handler) k8sClient(machine *capi.Machine) (kubernetes.Interface, error) {
	secret, err := h.secrets.Get(machine.Namespace, name.SafeConcatName(machine.Spec.ClusterName, "kubeconfig"))
	if err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restConfig)
}

func (h *handler) undrain(machine *capi.Machine, drainData string) (*capi.Machine, error) {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		logrus.Debugf("unable to drain machine %s as there is no noderef", machine.Name)
		return machine, nil
	}

	var drainOpts rkev1.DrainOptions
	if err := json.Unmarshal([]byte(drainData), &drainOpts); err != nil {
		return machine, err
	}

	if len(drainOpts.PostDrainHooks) > 0 {
		if machine.Annotations[planner.PostDrainAnnotation] != drainData {
			machine.Annotations[planner.PostDrainAnnotation] = drainData
			return h.machines.Update(machine)
		}
		for _, hook := range drainOpts.PostDrainHooks {
			if hook.Annotation != "" && machine.Annotations[hook.Annotation] != drainData {
				return machine, nil
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
	machine = machine.DeepCopy()
	delete(machine.Annotations, planner.PreDrainAnnotation)
	delete(machine.Annotations, planner.PostDrainAnnotation)
	delete(machine.Annotations, planner.DrainAnnotation)
	delete(machine.Annotations, planner.DrainDoneAnnotation)
	delete(machine.Annotations, planner.UnCordonAnnotation)
	for _, hook := range drainOpts.PreDrainHooks {
		delete(machine.Annotations, hook.Annotation)
	}
	for _, hook := range drainOpts.PostDrainHooks {
		delete(machine.Annotations, hook.Annotation)
	}
	return h.machines.Update(machine)
}

func (h *handler) drain(machine *capi.Machine, drainData string) (*capi.Machine, error) {
	drainOpts := &rkev1.DrainOptions{}
	if err := json.Unmarshal([]byte(drainData), drainOpts); err != nil {
		return nil, err
	}

	if err := h.cordon(machine, drainOpts); err != nil {
		return machine, err
	}

	if len(drainOpts.PreDrainHooks) > 0 {
		if machine.Annotations[planner.PreDrainAnnotation] != drainData {
			machine.Annotations[planner.PreDrainAnnotation] = drainData
			return h.machines.Update(machine)
		}
		for _, hook := range drainOpts.PreDrainHooks {
			if hook.Annotation != "" && machine.Annotations[hook.Annotation] != drainData {
				return machine, nil
			}
		}
	}

	if drainOpts.Enabled {
		if err := h.performDrain(machine, drainOpts); err != nil {
			return nil, err
		}
	}

	machine = machine.DeepCopy()
	machine.Annotations[planner.DrainDoneAnnotation] = drainData
	return h.machines.Update(machine)
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
