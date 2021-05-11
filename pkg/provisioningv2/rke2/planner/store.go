package planner

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	NoPlanPlanStatus         PlanStatus = "NoPlan"
	NoPlanPlanStatusMessage             = "waiting for plan to be assigned"
	WaitingPlanStatus        PlanStatus = "Waiting"
	WaitingPlanStatusMessage            = "waiting for plan to be applied"
	InSyncPlanStatus         PlanStatus = "InSync"
	InSyncPlanStatusMessage             = "plan applied"
	ErrorStatus              PlanStatus = "Error"
)

type PlanStatus string

type PlanStore struct {
	secrets      corecontrollers.SecretClient
	secretsCache corecontrollers.SecretCache
	machineCache capicontrollers.MachineCache
}

func NewStore(secrets corecontrollers.SecretController,
	machineCache capicontrollers.MachineCache) *PlanStore {
	return &PlanStore{
		secrets:      secrets,
		secretsCache: secrets.Cache(),
		machineCache: machineCache,
	}
}

func onlyRKE(machines []*capi.Machine) (result []*capi.Machine) {
	for _, m := range machines {
		if !isRKEBootstrap(m) {
			continue
		}
		result = append(result, m)
	}
	return
}

func (p *PlanStore) Load(cluster *capi.Cluster) (*plan.Plan, error) {
	result := &plan.Plan{
		Nodes:    map[string]*plan.Node{},
		Machines: map[string]*capi.Machine{},
		Cluster:  cluster,
	}

	machines, err := p.machineCache.List(cluster.Namespace, labels.SelectorFromSet(map[string]string{
		capiMachineLabel: cluster.Name,
	}))
	if err != nil {
		return nil, err
	}

	machines = onlyRKE(machines)

	secrets, err := p.getSecrets(machines)
	if err != nil {
		return nil, err
	}

	for _, machine := range machines {
		result.Machines[machine.Name] = machine
	}

	for machineName, secret := range secrets {
		node, err := SecretToNode(secret)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		result.Nodes[machineName] = node
	}

	return result, nil
}

func GetPlanStatusReasonMessage(plan *plan.Node) (corev1.ConditionStatus, PlanStatus, string) {
	switch {
	case len(plan.Plan.Instructions) == 0:
		return corev1.ConditionUnknown, NoPlanPlanStatus, NoPlanPlanStatusMessage
	case plan.Plan.Error != "":
		return corev1.ConditionFalse, ErrorStatus, plan.Plan.Error
	case plan.InSync:
		return corev1.ConditionTrue, InSyncPlanStatus, InSyncPlanStatusMessage
	default:
		return corev1.ConditionUnknown, WaitingPlanStatus, WaitingPlanStatusMessage
	}
}

func SecretToNode(secret *corev1.Secret) (*plan.Node, error) {
	result := &plan.Node{}
	planData := secret.Data["plan"]
	appliedPlanData := secret.Data["appliedPlan"]
	output := secret.Data["applied-output"]

	if len(planData) > 0 {
		if err := json.Unmarshal(planData, &result.Plan); err != nil {
			return nil, err
		}
	} else {
		return nil, nil
	}

	if len(appliedPlanData) > 0 {
		newPlan := &plan.NodePlan{}
		if err := json.Unmarshal(appliedPlanData, newPlan); err != nil {
			return nil, err
		}
		result.AppliedPlan = newPlan
	}

	if len(output) > 0 {
		gz, err := gzip.NewReader(bytes.NewBuffer(output))
		if err != nil {
			return nil, err
		}
		output, err = ioutil.ReadAll(gz)
		if err != nil {
			return nil, err
		}
		result.Output = map[string][]byte{}
		if err := json.Unmarshal(output, &result.Output); err != nil {
			return nil, err
		}
	}

	result.InSync = bytes.Equal(planData, appliedPlanData)
	return result, nil
}

func (p *PlanStore) getSecrets(machines []*capi.Machine) (map[string]*corev1.Secret, error) {
	result := map[string]*corev1.Secret{}
	for _, machine := range machines {
		secret, err := p.secretsCache.Get(machine.Namespace, PlanSecretFromBootstrapName(machine.Spec.Bootstrap.ConfigRef.Name))
		if apierror.IsNotFound(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		result[machine.Name] = secret
	}

	return result, nil
}

func isRKEBootstrap(machine *capi.Machine) bool {
	return machine.Spec.Bootstrap.ConfigRef != nil &&
		machine.Spec.Bootstrap.ConfigRef.Kind == "RKEBootstrap"
}

func (p *PlanStore) UpdatePlan(machine *capi.Machine, plan plan.NodePlan) error {
	if !isRKEBootstrap(machine) {
		return fmt.Errorf("machine %s/%s is not using RKEBootstrap", machine.Namespace, machine.Name)
	}

	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}

	secret, err := p.secrets.Get(machine.Namespace, PlanSecretFromBootstrapName(machine.Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	secret.Data["plan"] = data
	_, err = p.secrets.Update(secret)
	return err
}

func assignAndCheckPlan(store *PlanStore, msg string, server planEntry, newPlan plan.NodePlan) error {
	if server.Plan == nil || !equality.Semantic.DeepEqual(server.Plan.Plan, newPlan) {
		if err := store.UpdatePlan(server.Machine, newPlan); err != nil {
			return err
		}
		return ErrWaiting(fmt.Sprintf("starting %s", msg))
	}
	if !server.Plan.InSync {
		return ErrWaiting(fmt.Sprintf("waiting for %s", msg))
	}
	return nil
}
