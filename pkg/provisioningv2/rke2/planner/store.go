package planner

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	NoAgentPlanStatus        = "NoAgent"
	NoAgentPlanStatusMessage = "waiting for agent to check in and apply initial plan"
	NoPlanPlanStatus         = "NoPlan"
	UnHealthyProbes          = "UnHealthyProbes"
	WaitingPlanStatus        = "Waiting"
	WaitingPlanStatusMessage = "waiting for plan to be applied"
	InSyncPlanStatus         = "InSync"
	InSyncPlanStatusMessage  = "plan applied"
	FailedPlanStatusMessage  = "failure while applying plan"
	ErrorStatus              = "Error"
)

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
		CapiMachineLabel: cluster.Name,
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

func noPlanMessage(machine *capi.Machine) string {
	if isEtcd(machine) {
		return "waiting for bootstrap etcd to be available"
	} else if isControlPlane(machine) {
		return "waiting for etcd to be available"
	} else {
		return "waiting for control plane to be available"
	}
}

func probesMessage(plan *plan.Node) string {
	var (
		unhealthy []string
	)
	for name, probe := range plan.ProbeStatus {
		if !probe.Healthy {
			unhealthy = append(unhealthy, name)
		}
	}
	sort.Strings(unhealthy)
	return "waiting on probes: " + strings.Join(unhealthy, ", ")
}

func GetPlanStatusReasonMessage(machine *capi.Machine, plan *plan.Node) (corev1.ConditionStatus, string, string) {
	switch {
	case plan == nil:
		return corev1.ConditionUnknown, NoPlanPlanStatus, noPlanMessage(machine)
	case plan.AppliedPlan == nil:
		return corev1.ConditionUnknown, NoAgentPlanStatus, NoAgentPlanStatusMessage
	case len(plan.Plan.Instructions) == 0:
		return corev1.ConditionUnknown, NoPlanPlanStatus, noPlanMessage(machine)
	case plan.Plan.Error != "":
		return corev1.ConditionFalse, ErrorStatus, plan.Plan.Error
	case !plan.Healthy:
		return corev1.ConditionUnknown, UnHealthyProbes, probesMessage(plan)
	case plan.InSync:
		return corev1.ConditionTrue, InSyncPlanStatus, InSyncPlanStatusMessage
	case plan.Failed:
		return corev1.ConditionFalse, ErrorStatus, FailedPlanStatusMessage
	default:
		return corev1.ConditionUnknown, WaitingPlanStatus, WaitingPlanStatusMessage
	}
}

func SecretToNode(secret *corev1.Secret) (*plan.Node, error) {
	result := &plan.Node{
		Healthy: true,
	}
	planData := secret.Data["plan"]
	appliedPlanData := secret.Data["appliedPlan"]
	output := secret.Data["applied-output"]
	probes := secret.Data["probe-statuses"]
	failureCount := secret.Data["failure-count"]
	maxFailures := secret.Data["max-failures"]

	if len(failureCount) > 0 && len(maxFailures) > 0 {
		failureCount, err := strconv.Atoi(string(failureCount))
		if err != nil {
			return nil, err
		}
		maxFailures, err := strconv.Atoi(string(maxFailures))
		if err != nil {
			return nil, err
		}
		if failureCount >= maxFailures {
			result.Failed = true
		} else {
			result.Failed = false
		}
	} else {
		result.Failed = false
	}

	if len(probes) > 0 {
		result.ProbeStatus = map[string]plan.ProbeStatus{}
		if err := json.Unmarshal(probes, &result.ProbeStatus); err != nil {
			return nil, err
		}
		for _, status := range result.ProbeStatus {
			if !status.Healthy {
				result.Healthy = false
			}
		}
	}

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

	result.InSync = result.Healthy && bytes.Equal(planData, appliedPlanData)
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

func (p *PlanStore) UpdatePlan(machine *capi.Machine, plan plan.NodePlan, maxFailures int) error {
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
	if maxFailures > 0 {
		secret.Data["max-failures"] = []byte(strconv.Itoa(maxFailures))
	}
	_, err = p.secrets.Update(secret)
	return err
}

func assignAndCheckPlan(store *PlanStore, msg string, server planEntry, newPlan plan.NodePlan, maxFailures int) error {
	if server.Plan == nil || !equality.Semantic.DeepEqual(server.Plan.Plan, newPlan) {
		if err := store.UpdatePlan(server.Machine, newPlan, maxFailures); err != nil {
			return err
		}
		return ErrWaiting(fmt.Sprintf("starting %s", msg))
	}
	if !server.Plan.InSync {
		return ErrWaiting(fmt.Sprintf("waiting for %s", msg))
	}
	if server.Plan.Failed {
		return fmt.Errorf("operation %s failed", msg)
	}
	return nil
}
