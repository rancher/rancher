package planner

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	NoAgentPlanStatusMessage = "agent to check in and apply initial plan"
	WaitingPlanStatusMessage = "plan to be applied"
	FailedPlanStatusMessage  = "failure while applying plan"
)

type PlanStore struct {
	secrets              corecontrollers.SecretClient
	secretsCache         corecontrollers.SecretCache
	machineCache         capicontrollers.MachineCache
	serviceAccountsCache corecontrollers.ServiceAccountCache
	rkeBootstrapCache    rkecontrollers.RKEBootstrapCache
}

func NewStore(secrets corecontrollers.SecretController, machineCache capicontrollers.MachineCache, serviceAccountsCache corecontrollers.ServiceAccountCache, rkeBootstrapCache rkecontrollers.RKEBootstrapCache) *PlanStore {
	return &PlanStore{
		secrets:              secrets,
		secretsCache:         secrets.Cache(),
		serviceAccountsCache: serviceAccountsCache,
		machineCache:         machineCache,
		rkeBootstrapCache:    rkeBootstrapCache,
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

func (p *PlanStore) Load(cluster *capi.Cluster, rkeControlPlane *rkev1.RKEControlPlane) (*plan.Plan, error) {
	result := &plan.Plan{
		Nodes:    map[string]*plan.Node{},
		Machines: map[string]*capi.Machine{},
		Metadata: map[string]*plan.Metadata{},
		Cluster:  cluster,
	}

	machines, err := p.machineCache.List(cluster.Namespace, labels.SelectorFromSet(map[string]string{
		capi.ClusterLabelName: cluster.Name,
	}))
	if err != nil {
		return nil, err
	}

	machines = onlyRKE(machines)

	secrets, err := p.getPlanSecrets(machines)
	if err != nil {
		return nil, err
	}

	for _, machine := range machines {
		result.Machines[machine.Name] = machine
	}

	for machineName, secret := range secrets {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
		result.Metadata[machineName] = &plan.Metadata{
			Labels:      secret.Labels,
			Annotations: secret.Annotations,
		}
		node, err := SecretToNode(secret)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}

		if err := p.setMachineJoinURL(&planEntry{Machine: result.Machines[machineName], Metadata: result.Metadata[machineName], Plan: node}, cluster, rkeControlPlane); err != nil {
			return nil, err
		}

		result.Nodes[machineName] = node
	}

	return result, nil
}

func noPlanMessage(entry *planEntry) string {
	if isEtcd(entry) {
		return "bootstrap etcd to be available"
	} else if isControlPlane(entry) {
		return "etcd to be available"
	} else {
		return "control plane to be available"
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
	return "probes: " + strings.Join(unhealthy, ", ")
}

func getPlanStatusReasonMessage(entry *planEntry) string {
	switch {
	case entry.Plan == nil:
		return noPlanMessage(entry)
	case entry.Plan.AppliedPlan == nil:
		return NoAgentPlanStatusMessage
	case len(entry.Plan.Plan.Instructions) == 0:
		return noPlanMessage(entry)
	case entry.Plan.Plan.Error != "":
		return entry.Plan.Plan.Error
	case !entry.Plan.Healthy:
		return probesMessage(entry.Plan)
	case entry.Plan.InSync:
		return ""
	case entry.Plan.Failed:
		return FailedPlanStatusMessage
	default:
		return WaitingPlanStatusMessage
	}
}

func SecretToNode(secret *corev1.Secret) (*plan.Node, error) {
	result := &plan.Node{
		Healthy: true,
	}
	planData := secret.Data["plan"]
	appliedPlanData := secret.Data["appliedPlan"]
	output := secret.Data["applied-output"]
	appliedPeriodicOutput := secret.Data["applied-periodic-output"]
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
		output, err = io.ReadAll(gz)
		if err != nil {
			return nil, err
		}
		result.Output = map[string][]byte{}
		if err := json.Unmarshal(output, &result.Output); err != nil {
			return nil, err
		}
	}

	if len(appliedPeriodicOutput) > 0 {
		gz, err := gzip.NewReader(bytes.NewBuffer(appliedPeriodicOutput))
		if err != nil {
			return nil, err
		}
		output, err = io.ReadAll(gz)
		if err != nil {
			return nil, err
		}
		result.PeriodicOutput = map[string]plan.PeriodicInstructionOutput{}
		if err := json.Unmarshal(output, &result.PeriodicOutput); err != nil {
			return nil, err
		}
	}

	result.InSync = result.Healthy && bytes.Equal(planData, appliedPlanData)
	return result, nil
}

// getPlanSecrets retrieves the plan secrets for the given list of machines
func (p *PlanStore) getPlanSecrets(machines []*capi.Machine) (map[string]*corev1.Secret, error) {
	result := map[string]*corev1.Secret{}
	for _, machine := range machines {
		secret, err := p.getPlanSecretFromMachine(machine)
		if apierror.IsNotFound(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		if secret != nil {
			result[machine.Name] = secret.DeepCopy()
		}
	}

	return result, nil
}

func isRKEBootstrap(machine *capi.Machine) bool {
	return machine.Spec.Bootstrap.ConfigRef != nil &&
		machine.Spec.Bootstrap.ConfigRef.Kind == "RKEBootstrap"
}

// getPlanSecretFromachine returns the plan secret from the secretsCache for the given machine, or an error if the plan secret is not available
func (p *PlanStore) getPlanSecretFromMachine(machine *capi.Machine) (*corev1.Secret, error) {
	if !isRKEBootstrap(machine) {
		return nil, fmt.Errorf("machine %s/%s is not using RKEBootstrap", machine.Namespace, machine.Name)
	}

	planSAs, err := p.serviceAccountsCache.List(machine.Namespace, labels.SelectorFromSet(map[string]string{
		rke2.MachineNameLabel: machine.Name,
		rke2.RoleLabel:        rke2.RolePlan,
	}))
	if err != nil {
		return nil, err
	}

	if len(planSAs) != 1 {
		// This is an unexpected state and there are too many service accounts
		return nil, fmt.Errorf("error while retrieving plan secret for machine %s/%s service account list length was not 1", machine.Namespace, machine.Name)
	}

	planSecretName, _, err := rke2.GetServiceAccountSecretNames(p.rkeBootstrapCache, machine.Name, planSAs[0])
	if err != nil {
		return nil, err
	}

	if planSecretName == "" {
		return nil, fmt.Errorf("plan secret was not yet assigned for service account %s/%s", planSAs[0].Namespace, planSAs[0].Name)
	}

	return p.secretsCache.Get(planSAs[0].Namespace, planSecretName)
}

// UpdatePlan should not be called directly as it will not block further progress if the plan is not in sync
func (p *PlanStore) UpdatePlan(entry *planEntry, plan plan.NodePlan, maxFailures int) error {
	secret, err := p.getPlanSecretFromMachine(entry.Machine)
	if err != nil {
		return err
	}

	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}

	secret = secret.DeepCopy()

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	// if there are no probes, clear the statuses of the probes so as to prevent false positives
	if len(plan.Probes) == 0 {
		delete(secret.Data, "probe-statuses")
	}

	secret.Data["plan"] = data
	if maxFailures > 0 {
		secret.Data["max-failures"] = []byte(strconv.Itoa(maxFailures))
	}
	_, err = p.secrets.Update(secret)
	return err
}

func (p *PlanStore) updatePlanSecretLabelsAndAnnotations(entry *planEntry) error {
	secret, err := p.getPlanSecretFromMachine(entry.Machine)
	if err != nil {
		return err
	}

	secret = secret.DeepCopy()
	rke2.CopyPlanMetadataToSecret(secret, entry.Metadata)

	_, err = p.secrets.Update(secret)
	return err
}

func (p *PlanStore) removePlanSecretLabel(entry *planEntry, key string) error {
	secret, err := p.getPlanSecretFromMachine(entry.Machine)
	if err != nil {
		return err
	}

	if _, ok := secret.Labels[key]; !ok {
		return nil
	}

	secret = secret.DeepCopy()
	delete(secret.Labels, key)
	_, err = p.secrets.Update(secret)
	return err
}

// assignAndCheckPlan assigns the given newPlan to the designated server in the planEntry, and will return nil if the plan is assigned and in sync.
func assignAndCheckPlan(store *PlanStore, msg string, server *planEntry, newPlan plan.NodePlan, maxFailures int) error {
	if server.Plan == nil || !equality.Semantic.DeepEqual(server.Plan.Plan, newPlan) {
		if err := store.UpdatePlan(server, newPlan, maxFailures); err != nil {
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

func (p *PlanStore) setMachineJoinURL(entry *planEntry, capiCluster *capi.Cluster, rkeControlPlane *rkev1.RKEControlPlane) error {
	var (
		err     error
		joinURL string
	)

	if IsEtcdOnlyInitNode(entry) {
		joinURL, err = getJoinURLFromOutput(entry, capiCluster, rkeControlPlane)
		if err != nil || joinURL == "" {
			return err
		}
	} else {
		if entry.Machine.Status.NodeInfo == nil {
			return nil
		}

		address := ""
		for _, machineAddress := range entry.Machine.Status.Addresses {
			switch machineAddress.Type {
			case capi.MachineInternalIP:
				address = machineAddress.Address
			case capi.MachineExternalIP:
				if address == "" {
					address = machineAddress.Address
				}
			}
		}

		joinURL = fmt.Sprintf("https://%s:%d", address, rke2.GetRuntimeSupervisorPort(rkeControlPlane.Spec.KubernetesVersion))
	}

	if joinURL != "" && entry.Metadata.Annotations[rke2.JoinURLAnnotation] != joinURL {
		entry.Metadata.Annotations[rke2.JoinURLAnnotation] = joinURL
		if err := p.updatePlanSecretLabelsAndAnnotations(entry); err != nil {
			return err
		}

		return generic.ErrSkip
	}

	return nil
}

func getJoinURLFromOutput(entry *planEntry, capiCluster *capi.Cluster, rkeControlPlane *rkev1.RKEControlPlane) (string, error) {
	if entry.Plan == nil || !IsEtcdOnlyInitNode(entry) {
		return "", nil
	}

	var address []byte
	if ca, ok := entry.Plan.PeriodicOutput["capture-address"]; ok && ca.ExitCode == 0 && ca.LastSuccessfulRunTime != "" {
		address = ca.Stdout
	} else {
		return "", nil
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
		return "", nil
	}

	dbInfo := &dbinfo{}
	if err := json.Unmarshal([]byte(str), dbInfo); err != nil {
		return "", err
	}

	if len(dbInfo.Members) == 0 {
		return "", nil
	}

	if len(dbInfo.Members[0].ClientURLs) == 0 {
		return "", nil
	}

	u, err := url.Parse(dbInfo.Members[0].ClientURLs[0])
	if err != nil {
		return "", err
	}

	if capiCluster.Spec.ControlPlaneRef == nil || rkeControlPlane == nil {
		return "", nil
	}

	return fmt.Sprintf("https://%s:%d", u.Hostname(), rke2.GetRuntimeSupervisorPort(rkeControlPlane.Spec.KubernetesVersion)), nil
}

type dbinfo struct {
	Members []member `json:"members,omitempty"`
}
type member struct {
	ClientURLs []string `json:"clientURLs,omitempty"`
}
