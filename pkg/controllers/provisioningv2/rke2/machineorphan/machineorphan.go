package machineorphan

import (
	"context"

	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/slice"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	machineFinalizer = "machine.cluster.x-k8s.io"
)

// This is to work around an issue in cluster API where machines can't be delete if the
// controller cluster is deleted

type handler struct {
	clusterCache capicontrollers.ClusterCache
	machines     capicontrollers.MachineClient
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterCache: clients.CAPI.Cluster().Cache(),
		machines:     clients.CAPI.Machine(),
	}
	clients.CAPI.Machine().OnChange(ctx, "machine-orphan", h.OnChange)
}

func (h *handler) OnChange(key string, machine *capi.Machine) (*capi.Machine, error) {
	if machine == nil || machine.Spec.ClusterName == "" || machine.DeletionTimestamp == nil {
		return machine, nil
	}

	_, err := h.clusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
	if apierror.IsNotFound(err) {
		if slice.ContainsString(machine.Finalizers, machineFinalizer) {
			var finalizer []string
			for _, v := range machine.Finalizers {
				if v != machineFinalizer {
					finalizer = append(finalizer, v)
				}
			}
			machine = machine.DeepCopy()
			machine.Finalizers = finalizer
			return h.machines.Update(machine)
		}
	}

	return machine, nil
}
