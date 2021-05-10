package planstatus

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	Provisioned = condition.Cond("Provisioned")
)

type handler struct {
	secrets              corecontrollers.SecretClient
	machines             capicontrollers.MachineClient
	machineCache         capicontrollers.MachineCache
	bootstrapCache       rkecontroller.RKEBootstrapCache
	capiClusterCache     capicontrollers.ClusterCache
	rkeControlPlaneCache rkecontroller.RKEControlPlaneCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		secrets:              clients.Core.Secret(),
		machines:             clients.CAPI.Machine(),
		machineCache:         clients.CAPI.Machine().Cache(),
		bootstrapCache:       clients.RKE.RKEBootstrap().Cache(),
		capiClusterCache:     clients.CAPI.Cluster().Cache(),
		rkeControlPlaneCache: clients.RKE.RKEControlPlane().Cache(),
	}
	clients.Core.Secret().OnChange(ctx, "plan-status", h.OnChange)
}

func (h *handler) setJoinURLFromOutput(machine *capi.Machine, nodePlan *plan.Node) error {
	if !planner.IsEtcdOnlyInitNode(machine) || machine.Annotations[planner.JoinURLAnnotation] != "" {
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
		planner.GetRuntimeSupervisorPort(rkeControlPlane.Spec.KubernetesVersion))
	machine = machine.DeepCopy()
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}
	machine.Annotations[planner.JoinURLAnnotation] = joinURL
	_, err = h.machines.Update(machine)
	return err
}

func (h *handler) updateMachineProvisionStatus(secret *corev1.Secret) error {
	machineName := secret.Labels[planner.MachineNameLabel]
	if machineName == "" {
		return nil
	}

	machine, err := h.machineCache.Get(secret.Namespace, machineName)
	if err != nil {
		return err
	}

	if machine.Spec.Bootstrap.ConfigRef == nil &&
		machine.Spec.Bootstrap.ConfigRef.Kind != "RKEBootstrap" {
		return nil
	}

	rkeBootstrap, err := h.bootstrapCache.Get(secret.Namespace, machine.Spec.Bootstrap.ConfigRef.Name)
	if err != nil {
		return err
	}

	// make sure there's no funny business going on here
	if planner.PlanSecretFromBootstrapName(rkeBootstrap.Name) != secret.Name {
		return nil
	}

	plan, err := planner.SecretToNode(secret)
	if err != nil {
		return err
	}

	if err := h.setJoinURLFromOutput(machine, plan); err != nil {
		return err
	}

	if planner.IsEtcdOnlyInitNode(machine) && machine.Annotations[planner.JoinURLAnnotation] == "" {
		address, ok := plan.Output["capture-address"]
		if ok {
			str := string(address)
			i := strings.Index(str, "{")
			if i >= 0 {

			}

		}
	}

	status, reason, message := planner.GetPlanStatusReasonMessage(plan)
	if corev1.ConditionStatus(Provisioned.GetStatus(machine)) != status ||
		Provisioned.GetReason(machine) != string(reason) ||
		Provisioned.GetMessage(machine) != message {
		machine := machine.DeepCopy()
		newCond := capi.Condition{
			Type:               capi.ConditionType(Provisioned),
			Status:             status,
			LastTransitionTime: metav1.Now(),
			Reason:             string(reason),
			Message:            message,
		}
		if status == corev1.ConditionFalse {
			newCond.Severity = capi.ConditionSeverityError
		} else {
			newCond.Severity = capi.ConditionSeverityInfo
		}

		set := false
		for i, cond := range machine.Status.Conditions {
			if string(cond.Type) == string(Provisioned) {
				set = true
				machine.Status.Conditions[i] = newCond
				break
			}
		}

		if !set {
			machine.Status.Conditions = append(machine.Status.Conditions, newCond)
		}

		_, err := h.machines.UpdateStatus(machine)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) OnChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Type != planner.SecretTypeMachinePlan || len(secret.Data) == 0 {
		return secret, nil
	}

	if err := h.updateMachineProvisionStatus(secret); err != nil {
		return secret, err
	}

	if len(secret.Data) == 0 {
		return secret, nil
	}

	appliedChecksum := string(secret.Data["applied-checksum"])
	plan := secret.Data["plan"]
	appliedPlan := secret.Data["appliedPlan"]

	if appliedChecksum == hash(plan) {
		if !bytes.Equal(plan, appliedPlan) {
			secret = secret.DeepCopy()
			secret.Data["appliedPlan"] = plan
			return h.secrets.Update(secret)
		}
	}

	return secret, nil
}

func hash(plan []byte) string {
	result := sha256.Sum256(plan)
	return hex.EncodeToString(result[:])
}

type dbinfo struct {
	Members []member `json:"members,omitempty"`
}
type member struct {
	ClientURLs []string `json:"clientURLs,omitempty"`
}
