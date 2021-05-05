package machinedrain

import (
	"context"
	"encoding/json"
	"os"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/drain"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
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
	if drain != "" {
		return h.drain(machine, []byte(drain))
	}

	if machine.Annotations[planner.UnCordonAnnotation] != "" {
		return h.undrain(machine)
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

func (h *handler) undrain(machine *capi.Machine) (*capi.Machine, error) {
	k8s, err := h.k8sClient(machine)
	if err != nil {
		return nil, err
	}

	node, err := k8s.CoreV1().Nodes().Get(h.ctx, machine.Status.NodeRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	c := drain.NewCordonHelper(node)

	if updateRequired := c.UpdateIfRequired(false); !updateRequired {
		return machine, nil
	}

	err, patchErr := c.PatchOrReplaceWithContext(h.ctx, k8s, false)
	if err != nil {
		return machine, err
	}

	return machine, patchErr
}

func (h *handler) drain(machine *capi.Machine, drainData []byte) (*capi.Machine, error) {
	hash := planner.DrainHash(drainData)
	if machine.Annotations[planner.DrainDoneAnnotation] == hash {
		return machine, nil
	}

	drainOpts := &rkev1.DrainOptions{}
	if err := json.Unmarshal(drainData, drainOpts); err != nil {
		return nil, err
	}

	if err := h.performDrain(machine, drainOpts); err != nil {
		return nil, err
	}

	machine = machine.DeepCopy()
	machine.Annotations[planner.DrainDoneAnnotation] = hash
	return h.machines.Update(machine)
}

func (h *handler) performDrain(machine *capi.Machine, drainOpts *rkev1.DrainOptions) error {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		return nil
	}

	k8s, err := h.k8sClient(machine)
	if err != nil {
		return err
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
		return err
	}

	if err := drain.RunCordonOrUncordon(helper, node, true); err != nil {
		return err
	}

	return drain.RunNodeDrain(helper, node.Name)
}
