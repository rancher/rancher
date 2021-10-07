package machinestatus

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	rancherruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	Provisioned         = condition.Cond("Provisioned")
	InfrastructureReady = condition.Cond(capi.InfrastructureReadyCondition)
	BootstrapReady      = condition.Cond(capi.BootstrapReadyCondition)
)

type handler struct {
	secrets              corecontrollers.SecretCache
	machines             capicontrollers.MachineController
	bootstrapCache       rkecontroller.RKEBootstrapCache
	bootstrapController  rkecontroller.RKEBootstrapController
	capiClusterCache     capicontrollers.ClusterCache
	provClusterCache     provisioningcontrollers.ClusterCache
	mgmtClusterCache     mgmtcontrollers.ClusterCache
	rkeControlPlaneCache rkecontroller.RKEControlPlaneCache
	dynamic              *dynamic.Controller
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		secrets:              clients.Core.Secret().Cache(),
		machines:             clients.CAPI.Machine(),
		bootstrapCache:       clients.RKE.RKEBootstrap().Cache(),
		bootstrapController:  clients.RKE.RKEBootstrap(),
		capiClusterCache:     clients.CAPI.Cluster().Cache(),
		mgmtClusterCache:     clients.Mgmt.Cluster().Cache(),
		provClusterCache:     clients.Provisioning.Cluster().Cache(),
		rkeControlPlaneCache: clients.RKE.RKEControlPlane().Cache(),
		dynamic:              clients.Dynamic,
	}
	clients.CAPI.Machine().OnChange(ctx, "machine-status", h.OnChange)

	relatedresource.Watch(ctx, "machine-trigger-from-secret", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if secret, ok := obj.(*corev1.Secret); ok {
			if secret.Type == planner.SecretTypeMachinePlan {
				return []relatedresource.Key{{
					Namespace: secret.Namespace,
					Name:      secret.Labels[planner.MachineNameLabel],
				}}, nil
			}
		}
		return nil, nil
	}, clients.CAPI.Machine(), clients.Core.Secret())

	h.dynamic.OnChange(ctx, "machine-trigger", func(gvk schema.GroupVersionKind) bool {
		return gvk.Group == "rke-machine.cattle.io"
	}, func(obj runtime.Object) (runtime.Object, error) {
		m, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		for _, owner := range m.GetOwnerReferences() {
			if owner.Kind == "Machine" {
				h.machines.Enqueue(m.GetNamespace(), owner.Name)
			}
		}
		return obj, nil
	})
}

func (h *handler) setJoinURLFromOutput(machine *capi.Machine, nodePlan *plan.Node) error {
	if nodePlan == nil || !planner.IsEtcdOnlyInitNode(machine) || machine.Annotations[planner.JoinURLAnnotation] != "" {
		return nil
	}

	address, ok := nodePlan.Output["capture-address"]
	if !ok {
		return nil
	}

	var str string
	scanner := bufio.NewScanner(bytes.NewBuffer(address))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "{") {
			str = line
			break
		}
	}

	if str == "" {
		return nil
	}

	dbInfo := &dbinfo{}
	if err := json.Unmarshal([]byte(str), dbInfo); err != nil {
		return err
	}

	if len(dbInfo.Members) == 0 {
		return nil
	}

	if len(dbInfo.Members[0].ClientURLs) == 0 {
		return nil
	}

	u, err := url.Parse(dbInfo.Members[0].ClientURLs[0])
	if err != nil {
		return err
	}

	cluster, err := h.capiClusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
	if err != nil {
		return err
	}

	if cluster.Spec.ControlPlaneRef == nil {
		return nil
	}

	rkeControlPlane, err := h.rkeControlPlaneCache.Get(machine.Namespace, cluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return err
	}

	joinURL := fmt.Sprintf("https://%s:%d", u.Hostname(),
		rancherruntime.GetRuntimeSupervisorPort(rkeControlPlane.Spec.KubernetesVersion))
	machine = machine.DeepCopy()
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}
	machine.Annotations[planner.JoinURLAnnotation] = joinURL
	_, err = h.machines.Update(machine)
	return err
}

func (h *handler) OnChange(_ string, machine *capi.Machine) (*capi.Machine, error) {
	if machine == nil ||
		machine.Spec.Bootstrap.ConfigRef == nil ||
		machine.Spec.Bootstrap.ConfigRef.Kind != "RKEBootstrap" {
		return machine, nil
	}

	status, err := h.getInfraMachineState(machine)
	if status.cond == InfrastructureReady {
		// If the machine is being deleted and the infrastructure machine object is not found or failed to delete,
		// then update the status of the machine object so the CAPI controller picks it up.
		return h.setMachineCondition(machine, status)
	} else if err != nil {
		return machine, err
	}

	rkeBootstrap, err := h.bootstrapCache.Get(machine.Namespace, machine.Spec.Bootstrap.ConfigRef.Name)
	if err != nil {
		if machine.DeletionTimestamp != nil && apierror.IsNotFound(err) {
			// If the machine is being deleted and the bootstrap object is not found,
			// then update the status of the machine object so the CAPI controller picks it up.
			return h.setMachineCondition(machine, &machineStatus{cond: BootstrapReady, status: corev1.ConditionFalse, reason: capi.DeletedReason, message: "bootstrap is deleted"})
		}
		return machine, err
	}

	secret, err := h.secrets.Get(machine.Namespace, planner.PlanSecretFromBootstrapName(rkeBootstrap.Name))
	if apierror.IsNotFound(err) {
		// When the secret exists this handler will be triggered, so don't error
		return machine, nil
	} else if err != nil {
		return machine, err
	}

	plan, err := planner.SecretToNode(secret)
	if err != nil {
		return machine, err
	}

	if err := h.setJoinURLFromOutput(machine, plan); err != nil {
		return machine, err
	}

	if status.status == "" {
		status.status, status.reason, status.message = planner.GetPlanStatusReasonMessage(machine, plan)
	}

	if status.status == corev1.ConditionTrue && status.providerID == "" {
		status.status = corev1.ConditionUnknown
		status.reason = "NoProviderID"
		status.message = "waiting for node to be registered in Kubernetes"
		provCluster, err := h.provClusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
		if err == nil {
			mgmtCluster, err := h.mgmtClusterCache.Get(provCluster.Status.ClusterName)
			if err == nil {
				if condition.Cond("Ready").IsTrue(mgmtCluster) {
					h.bootstrapController.Enqueue(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name)
				} else if planner.IsOnlyEtcd(machine) {
					status.message = "waiting for cluster agent to be available on a control plane node"
					h.machines.EnqueueAfter(machine.Namespace, machine.Name, 2*time.Second)
				} else {
					status.message = "waiting for cluster agent to be available"
					h.machines.EnqueueAfter(machine.Namespace, machine.Name, 2*time.Second)
				}
			}
		}
	}

	return h.setMachineCondition(machine, status)
}

type dbinfo struct {
	Members []member `json:"members,omitempty"`
}
type member struct {
	ClientURLs []string `json:"clientURLs,omitempty"`
}
