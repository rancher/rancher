package planner

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/utils"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	NoAgentPlanStatusMessage = "waiting for agent to check in and apply initial plan"
	WaitingPlanStatusMessage = "waiting for plan to be applied"
	FailedPlanStatusMessage  = "failure while applying plan"
)

type PlanStore struct {
	secrets      corecontrollers.SecretClient
	secretsCache corecontrollers.SecretCache
	machineCache capicontrollers.MachineCache
}

func NewStore(secrets corecontrollers.SecretController, machineCache capicontrollers.MachineCache) *PlanStore {
	return &PlanStore{
		secrets:      secrets,
		secretsCache: secrets.Cache(),
		machineCache: machineCache,
	}
}

// onlyRKE returns a subset of the passed in slice of CAPI machines that only contains machines that have an RKEBootstrap
func onlyRKE(machines []*capi.Machine) (result []*capi.Machine) {
	for _, m := range machines {
		if !isRKEBootstrap(m) {
			continue
		}
		result = append(result, m)
	}
	return
}

// Load takes a clusters.cluster.x-k8s.io object and the corresponding rkecontrolplanes.rke.cattle.io object and
// generates a new plan.Plan, a bool that indicates whether any plan has been delivered to any of the machines,
// and an error
func (p *PlanStore) Load(cluster *capi.Cluster, rkeControlPlane *rkev1.RKEControlPlane) (*plan.Plan, bool, error) {
	result := &plan.Plan{
		Nodes:    map[string]*plan.Node{},
		Machines: map[string]*capi.Machine{},
		Metadata: map[string]*plan.Metadata{},
	}

	var anyPlanDelivered bool

	machines, err := p.machineCache.List(cluster.Namespace, labels.SelectorFromSet(map[string]string{
		capi.ClusterNameLabel: cluster.Name,
	}))
	if err != nil {
		return nil, anyPlanDelivered, err
	}

	machines = onlyRKE(machines)

	secrets, err := p.getPlanSecrets(machines)
	if err != nil {
		return nil, anyPlanDelivered, err
	}

	// Place a DeepCopy of the machine in the resulting Plan, making mutative operations on the Machine object safe.
	for _, machine := range machines {
		result.Machines[machine.Name] = machine.DeepCopy()
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
			return nil, anyPlanDelivered, err
		}
		if node == nil {
			continue
		}
		if node.PlanDataExists {
			anyPlanDelivered = true
		}
		if err := p.setMachineJoinURL(&planEntry{Machine: result.Machines[machineName], Metadata: result.Metadata[machineName], Plan: node}, cluster, rkeControlPlane); err != nil {
			return nil, anyPlanDelivered, err
		}
		result.Nodes[machineName] = node
	}

	return result, anyPlanDelivered, nil
}

func noPlanMessage(entry *planEntry) string {
	if isEtcd(entry) {
		return "waiting for bootstrap etcd to be available"
	}
	if isControlPlane(entry) {
		return "waiting for etcd to be available"
	}
	return "waiting for control plane to be available"
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
	return "waiting for probes: " + strings.Join(unhealthy, ", ")
}

func getPlanStatusReasonMessage(entry *planEntry) string {
	switch {
	case entry.Plan == nil:
		return noPlanMessage(entry)
	case len(entry.Plan.Plan.Instructions) == 0:
		return noPlanMessage(entry)
	case entry.Plan.AppliedPlan == nil:
		return NoAgentPlanStatusMessage
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

// SecretToNode consumes a secret of type rke.cattle.io/machine-plan and returns a node object and an error if one exists
func SecretToNode(secret *corev1.Secret) (*plan.Node, error) {
	if secret == nil {
		return nil, fmt.Errorf("unable to convert secret to node plan, secret was nil")
	}
	if secret.Type != capr.SecretTypeMachinePlan {
		return nil, fmt.Errorf("secret %s/%s was not type %s", secret.Namespace, secret.Name, capr.SecretTypeMachinePlan)
	}
	result := &plan.Node{
		Healthy: true,
	}

	planData := secret.Data["plan"]
	result.PlanDataExists = len(secret.Data["plan"]) != 0
	appliedPlanData := secret.Data["appliedPlan"]
	failedChecksum := string(secret.Data["failed-checksum"])
	output := secret.Data["applied-output"]
	appliedPeriodicOutput := secret.Data["applied-periodic-output"]
	probes := secret.Data["probe-statuses"]
	failureCount := secret.Data["failure-count"]

	if probesPassed, ok := secret.Annotations[capr.PlanProbesPassedAnnotation]; ok && probesPassed != "" {
		result.ProbesUsable = true
	}

	if len(failureCount) > 0 && PlanHash(planData) == failedChecksum {
		failureCount, err := strconv.Atoi(string(failureCount))
		if err != nil {
			return nil, err
		}
		if failureCount > 0 {
			result.Failed = true
			// failure-threshold is set by Rancher when the plan is updated. If it is not set, then it essentially
			// defaults to 1, and any failure causes the plan to be marked as failed
			rawFailureThreshold := secret.Data["failure-threshold"]
			if len(rawFailureThreshold) > 0 {
				failureThreshold, err := strconv.Atoi(string(rawFailureThreshold))
				if err != nil {
					return nil, err
				}
				if failureCount < failureThreshold || failureThreshold == -1 {
					// the plan hasn't actually failed to be applied because we haven't passed the failure threshold or failure threshold is set to -1.
					result.Failed = false
				}
			}
		}
	}

	if len(probes) > 0 {
		probeStatuses, healthy, err := ParseProbeStatuses(probes)
		if err != nil {
			return nil, err
		}
		result.ProbeStatus = *probeStatuses
		result.Healthy = healthy
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

	if joinedTo, ok := secret.Annotations[capr.JoinedToAnnotation]; ok {
		result.JoinedTo = joinedTo
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

	result.InSync = bytes.Equal(planData, appliedPlanData)
	return result, nil
}

func ParseProbeStatuses(probeStatuses []byte) (*map[string]plan.ProbeStatus, bool, error) {
	healthy := true
	if len(probeStatuses) == 0 {
		return nil, false, fmt.Errorf("probe status length was 0")
	}
	probeStatusMap := map[string]plan.ProbeStatus{}
	if err := json.Unmarshal(probeStatuses, &probeStatusMap); err != nil {
		return nil, false, err
	}
	for _, status := range probeStatusMap {
		if !status.Healthy {
			healthy = false
		}
	}
	return &probeStatusMap, healthy, nil
}

func PlanHash(plan []byte) string {
	result := sha256.Sum256(plan)
	return hex.EncodeToString(result[:])
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

// getPlanSecretFromMachine returns the plan secret from the secrets client for the given machine,
// or an error if the plan secret is not available. Notably we do not use the secretsCache to ensure we only operate
// on the latest version of a machine plan secret.
func (p *PlanStore) getPlanSecretFromMachine(machine *capi.Machine) (*corev1.Secret, error) {
	if machine == nil {
		return nil, fmt.Errorf("machine was nil")
	}

	if !isRKEBootstrap(machine) {
		return nil, fmt.Errorf("machine %s/%s is not using RKEBootstrap", machine.Namespace, machine.Name)
	}

	if machine.Spec.Bootstrap.ConfigRef == nil {
		return nil, fmt.Errorf("machine %s/%s bootstrap configref was nil", machine.Namespace, machine.Name)
	}

	if machine.Spec.Bootstrap.ConfigRef.Name == "" {
		return nil, fmt.Errorf("machine %s/%s bootstrap configref name was empty", machine.Namespace, machine.Name)
	}

	secret, err := p.secrets.Get(machine.Namespace, capr.PlanSecretFromBootstrapName(machine.Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if secret.Type != capr.SecretTypeMachinePlan {
		return nil, fmt.Errorf("retrieved secret %s/%s type %s did not match expected type %s", secret.Namespace, secret.Name, secret.Type, capr.SecretTypeMachinePlan)
	}

	return secret, nil
}

// UpdatePlan should not be called directly as it will not block further progress if the plan is not in sync
// maxFailures is the number of attempts the system-agent will make to run the plan (in a failed state). failureThreshold is used to determine when the plan has failed.
func (p *PlanStore) UpdatePlan(entry *planEntry, newNodePlan plan.NodePlan, joinedTo string, maxFailures, failureThreshold int) error {
	if maxFailures < failureThreshold && failureThreshold != -1 && maxFailures != -1 {
		return fmt.Errorf("failureThreshold (%d) cannot be greater than maxFailures (%d)", failureThreshold, maxFailures)
	}
	secret, err := p.getPlanSecretFromMachine(entry.Machine)
	if err != nil {
		return err
	}

	data, err := json.Marshal(newNodePlan)
	if err != nil {
		return err
	}

	secret = secret.DeepCopy()
	if secret.Data == nil {
		// Create the map with enough storage for what is needed.
		secret.Data = make(map[string][]byte, 6)
	}

	// If joinedTo is specified, set the joined-to annotation. If -, then clear the joined-to annotation
	if joinedTo != "" {
		if joinedTo == "-" || entry.Metadata.Annotations[capr.InitNodeLabel] == "true" {
			// clear the joinedTo annotation.
			entry.Metadata.Annotations[capr.JoinedToAnnotation] = ""
		} else {
			entry.Metadata.Annotations[capr.JoinedToAnnotation] = joinedTo
		}
	}

	// an init node cannot have a joined-to annotation value as it is essentially joined to itself.
	if entry.Metadata.Annotations[capr.InitNodeLabel] == "true" {
		entry.Metadata.Annotations[capr.JoinedToAnnotation] = ""
	}

	entry.Metadata.Annotations[capr.PlanUpdatedTimeAnnotation] = time.Now().UTC().Format(time.RFC3339)
	entry.Metadata.Annotations[capr.PlanProbesPassedAnnotation] = ""

	capr.CopyPlanMetadataToSecret(secret, entry.Metadata)

	// If the plan is being updated, then delete the probe-statuses so their healthy status will be reported as healthy only when they pass.
	delete(secret.Data, "probe-statuses")

	secret.Data["plan"] = data
	if maxFailures > 0 || maxFailures == -1 {
		secret.Data["max-failures"] = []byte(strconv.Itoa(maxFailures))
	} else {
		delete(secret.Data, "max-failures")
	}

	if failureThreshold > 0 || failureThreshold == -1 {
		secret.Data["failure-threshold"] = []byte(strconv.Itoa(failureThreshold))
	} else {
		delete(secret.Data, "failure-threshold")
	}

	updatedSecret, err := p.secrets.Update(secret)
	if err != nil {
		return err
	}

	// Update the node immediately so that future plan processing occurs
	newNode, err := SecretToNode(updatedSecret)
	if err != nil {
		return err
	}

	entry.Plan = newNode
	return nil
}

func (p *PlanStore) updatePlanSecretLabelsAndAnnotations(entry *planEntry) error {
	secret, err := p.getPlanSecretFromMachine(entry.Machine)
	if err != nil {
		return err
	}

	secret = secret.DeepCopy()
	capr.CopyPlanMetadataToSecret(secret, entry.Metadata)
	updatedSecret, err := p.secrets.Update(secret)
	if err != nil {
		return err
	}
	newNode, err := SecretToNode(updatedSecret)
	if err != nil {
		return err
	}
	entry.Plan = newNode
	return nil
}

// removePlanSecretLabel removes a label with the given key from the plan secret that corresponds to the RKEBootstrap
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
	updatedSecret, err := p.secrets.Update(secret)
	if err != nil {
		return err
	}
	newNode, err := SecretToNode(updatedSecret)
	if err != nil {
		return err
	}
	entry.Plan = newNode
	entry.Metadata.Labels = secret.Labels
	return nil
}

// assignAndCheckPlan assigns the given newPlan to the designated server in the planEntry, and will return nil if the plan is assigned and in sync.
func assignAndCheckPlan(store *PlanStore, msg string, entry *planEntry, newPlan plan.NodePlan, joinedTo string, failureThreshold, maxRetries int) error {
	if entry.Plan == nil || !equality.Semantic.DeepEqual(entry.Plan.Plan, newPlan) {
		if err := store.UpdatePlan(entry, newPlan, joinedTo, failureThreshold, maxRetries); err != nil {
			return err
		}
		return errWaiting(fmt.Sprintf("starting %s", msg))
	}
	if entry.Plan.Failed {
		return fmt.Errorf("operation %s failed", msg)
	}
	if !entry.Plan.InSync {
		return errWaiting(fmt.Sprintf("waiting for %s", msg))
	}
	if !entry.Plan.Healthy {
		return errWaiting(fmt.Sprintf("waiting for %s probes", msg))
	}
	return nil
}

// setMachineJoinURL determines and updates a machine plan secret with the join URL/joined URL for the provided node.
func (p *PlanStore) setMachineJoinURL(entry *planEntry, capiCluster *capi.Cluster, controlPlane *rkev1.RKEControlPlane) error {
	var (
		err     error
		joinURL string
	)

	// If the annotation to disable autosetting the join URL is enabled, don't process the join URL.
	if _, autosetDisabled := entry.Metadata.Annotations[capr.JoinURLAutosetDisabled]; autosetDisabled {
		return nil
	}

	if IsEtcdOnlyInitNode(entry) {
		joinURL, err = getJoinURLFromOutput(entry, capiCluster, controlPlane)
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
		if address != "" {
			joinURL = joinURLFromAddress(address, capr.GetRuntimeSupervisorPort(controlPlane.Spec.KubernetesVersion))
		}
	}

	updateRequired := false

	if joinURL != "" && entry.Metadata.Annotations[capr.JoinURLAnnotation] != joinURL {
		entry.Metadata.Annotations[capr.JoinURLAnnotation] = joinURL
		updateRequired = true
	}

	// In certain cases, it is possible that the joined-to annotation is not set. Attempt to determine the joined to
	// annotation based on the delivered plan.
	if !isInitNode(entry) && entry.Metadata.Annotations[capr.JoinedToAnnotation] == "" {
		if configFirstHalf, configSecondHalf, found := strings.Cut(ConfigYamlFileName, "%s"); found {
			for _, v := range entry.Plan.Plan.Files {
				if strings.Contains(v.Path, configFirstHalf) && strings.Contains(v.Path, configSecondHalf) {
					// We found our config file, process it to look for the joined node and then break
					cfr, err := base64.StdEncoding.DecodeString(v.Content)
					if err != nil {
						return err
					}
					var cf = map[string]interface{}{}
					if err := json.Unmarshal(cfr, &cf); err != nil {
						return err
					}
					if server, ok := cf["server"]; ok {
						entry.Metadata.Annotations[capr.JoinedToAnnotation] = server.(string)
						updateRequired = true
					}
					break
				}
			}
		}
	}

	if updateRequired {
		if err := p.updatePlanSecretLabelsAndAnnotations(entry); err != nil {
			return err
		}
		return generic.ErrSkip
	}

	return nil
}

// joinURLFromAddress accepts both an address (IPv4, IPv6, hostname) and returns a fully rendered join URL including `https://` and the supervisor port
func joinURLFromAddress(address string, port int) string {
	// ipv6 addresses need to be enclosed in brackets in URLs, and hostnames will fail to be parsed as IPs
	if utils.IsPlainIPV6(address) {
		address = fmt.Sprintf("[%s]", address)
	}
	return fmt.Sprintf("https://%s:%d", address, port)
}

// getJoinURLFromOutput parses the periodic output from a given entry and determines the full join URL including `https://` and the supervisor port
func getJoinURLFromOutput(entry *planEntry, capiCluster *capi.Cluster, rkeControlPlane *rkev1.RKEControlPlane) (string, error) {
	if entry.Plan == nil || !IsEtcdOnlyInitNode(entry) || capiCluster.Spec.ControlPlaneRef == nil || rkeControlPlane == nil {
		return "", nil
	}

	var address []byte
	var name string
	ca := entry.Plan.PeriodicOutput[captureAddressInstructionName]
	if ca.ExitCode != 0 || ca.LastSuccessfulRunTime == "" {
		return "", nil
	}
	etcdNameOutput := entry.Plan.PeriodicOutput[etcdNameInstructionName]
	if etcdNameOutput.ExitCode != 0 || etcdNameOutput.LastSuccessfulRunTime == "" {
		return "", nil
	}
	address = ca.Stdout
	name = string(bytes.TrimSpace(etcdNameOutput.Stdout))

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

	for _, member := range dbInfo.Members {
		if member.Name != name {
			continue
		}

		u, err := url.Parse(member.ClientURLs[0])
		if err != nil {
			return "", err
		}

		joinURL := joinURLFromAddress(u.Hostname(), capr.GetRuntimeSupervisorPort(rkeControlPlane.Spec.KubernetesVersion))
		return joinURL, nil
	}

	// No need to error here because once the plan secret is updated, then this will be retried.
	return "", nil
}

type dbinfo struct {
	Members []member `json:"members,omitempty"`
}
type member struct {
	Name       string   `json:"name,omitempty"`
	ClientURLs []string `json:"clientURLs,omitempty"`
}
