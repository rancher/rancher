package plansecret

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

type handler struct {
	secrets              corecontrollers.SecretClient
	machinesCache        capicontrollers.MachineCache
	machinesClient       capicontrollers.MachineClient
	etcdSnapshotsClient  rkev1controllers.ETCDSnapshotClient
	etcdSnapshotsCache   rkev1controllers.ETCDSnapshotCache
	rkeControlPlaneCache rkev1controllers.RKEControlPlaneCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		secrets:              clients.Core.Secret(),
		machinesCache:        clients.CAPI.Machine().Cache(),
		machinesClient:       clients.CAPI.Machine(),
		etcdSnapshotsClient:  clients.RKE.ETCDSnapshot(),
		etcdSnapshotsCache:   clients.RKE.ETCDSnapshot().Cache(),
		rkeControlPlaneCache: clients.RKE.RKEControlPlane().Cache(),
	}
	clients.Core.Secret().OnChange(ctx, "plan-secret", h.OnChange)
}

func (h *handler) OnChange(_ string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Type != capr.SecretTypeMachinePlan || len(secret.Data) == 0 {
		return secret, nil
	}
	var err error

	logrus.Debugf("[plansecret] reconciling secret %s/%s", secret.Namespace, secret.Name)

	appliedChecksum := string(secret.Data["applied-checksum"])
	failedChecksum := string(secret.Data["failed-checksum"])
	plan := secret.Data["plan"]

	failureCount := string(secret.Data["failure-count"])
	maxFailures := string(secret.Data["max-failures"])

	secretChanged := false
	secret = secret.DeepCopy()

	if appliedChecksum == planner.PlanHash(plan) && !bytes.Equal(plan, secret.Data["appliedPlan"]) {
		secret.Data["appliedPlan"] = plan
		secretChanged = true
	}

	if len(secret.Data["probe-statuses"]) > 0 {
		_, healthy, err := planner.ParseProbeStatuses(secret.Data["probe-statuses"])
		if err != nil {
			return nil, err
		}
		if healthy && secret.Annotations[capr.PlanProbesPassedAnnotation] == "" {
			// a non-zero value for this annotation indicates the probes for this specific plan have passed at least once
			secret.Annotations[capr.PlanProbesPassedAnnotation] = time.Now().UTC().Format(time.RFC3339)
			secretChanged = true
		}
	}

	node, err := planner.SecretToNode(secret)
	if err != nil {
		return nil, err
	}

	if purgePeriodicInstructionOutput(node) {
		if len(node.PeriodicOutput) == 0 {
			// special case for when there are no remaining periodic instructions
			secret.Data["applied-periodic-output"] = []byte{}
		} else {
			input, err := json.Marshal(node.PeriodicOutput)
			if err != nil {
				return nil, err
			}

			var output bytes.Buffer

			gz := gzip.NewWriter(&output)
			if _, err = gz.Write(input); err != nil {
				return nil, err
			}
			if err = gz.Close(); err != nil {
				return nil, err
			}

			secret.Data["applied-periodic-output"] = output.Bytes()
		}
		secretChanged = true
	}

	if secretChanged {
		// don't return the secret at this point, we want to attempt to update the machine status later on
		secret, err = h.secrets.Update(secret)
		if err != nil {
			return secret, err
		}
	}

	if failedChecksum == planner.PlanHash(plan) {
		logrus.Debugf("[plansecret] %s/%s: rv: %s: Detected failed plan application, reconciling machine PlanApplied condition to error", secret.Namespace, secret.Name, secret.ResourceVersion)
		// plans which temporarily fail will continue to set the failedChecksum as expected, however this should not be considered a
		// true failure unless we have required that the plan not fail at any point, or we have reached the maximum of attempts configured.
		// After a successful application, the checksum is cleared by the system-agent.
		if maxFailures == "-1" || failureCount == maxFailures {
			var andRuntimeUnit string
			if clusterName, ok := secret.Labels[capr.ClusterNameLabel]; ok && len(clusterName) > 0 {
				if controlPlane, err := h.rkeControlPlaneCache.Get(secret.Namespace, clusterName); err != nil {
					logrus.Errorf("unable to get RKEControlPlane (%s/%s) for plan secret (%s/%s): %v", secret.Namespace, clusterName, secret.Namespace, secret.Name, err)
				} else {
					var runtimeUnit string
					if secret.Labels[capr.ControlPlaneRoleLabel] == "true" || secret.Labels[capr.EtcdRoleLabel] == "true" {
						runtimeUnit = capr.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion)
					} else {
						runtimeUnit = capr.GetRuntimeAgentUnit(controlPlane.Spec.KubernetesVersion)
					}
					if runtimeUnit != "" {
						andRuntimeUnit = fmt.Sprintf(" and %s.service", runtimeUnit)
					}
				}
			}
			err = h.reconcileMachinePlanAppliedCondition(secret, fmt.Errorf("error applying plan -- check rancher-system-agent.service%s logs on node for more information", andRuntimeUnit))
		}
		return secret, err
	}

	logrus.Debugf("[plansecret] %s/%s: rv: %s: Reconciling machine PlanApplied condition to nil", secret.Namespace, secret.Name, secret.ResourceVersion)
	err = h.reconcileMachinePlanAppliedCondition(secret, nil)
	return secret, err
}

// purgePeriodicInstructionOutput will parse the node plan and remove the periodic output for instructions which are no
// longer present in the current plan. Returns "" when the entire list is removed, otherwise it returns the json blob of
// remaining keys gzipped for setting the "applied-periodic-output". Returns nil only when the "applied-periodic-output"
// key is not to be overwritten. This function modifies the node plan in place.
func purgePeriodicInstructionOutput(node *plan.Node) bool {
	if node == nil {
		return false
	}

	// if old periodic instructions are within the plan, remove them
	knownInstructions := map[string]struct{}{}
	for _, p := range node.Plan.PeriodicInstructions {
		knownInstructions[p.Name] = struct{}{}
	}

	removed := false
	for n := range node.PeriodicOutput {
		if _, ok := knownInstructions[n]; !ok {
			// remove from periodic output
			delete(node.PeriodicOutput, n)
			removed = true
		}
	}

	return removed
}

func (h *handler) reconcileMachinePlanAppliedCondition(secret *corev1.Secret, planAppliedErr error) error {
	if secret == nil {
		logrus.Debug("[plansecret] secret was nil when reconciling machine status")
		return nil
	}

	condition := capi.ConditionType(capr.PlanApplied)

	machineName, ok := secret.Labels[capr.MachineNameLabel]
	if !ok {
		return fmt.Errorf("did not find machine label on secret %s/%s", secret.Namespace, secret.Name)
	}

	machine, err := h.machinesCache.Get(secret.Namespace, machineName)
	if err != nil {
		return err
	}

	machine = machine.DeepCopy()

	var needsUpdate bool
	if planAppliedErr != nil &&
		(conditions.GetMessage(machine, condition) != planAppliedErr.Error() ||
			*conditions.GetSeverity(machine, condition) != capi.ConditionSeverityError ||
			!conditions.IsFalse(machine, condition) ||
			conditions.GetReason(machine, condition) != "Error") {
		logrus.Debugf("[plansecret] machine %s/%s: marking PlanApplied as false", machine.Namespace, machine.Name)
		conditions.MarkFalse(machine, condition, "Error", capi.ConditionSeverityError, "%s", planAppliedErr.Error())
		needsUpdate = true
	} else if planAppliedErr == nil && !conditions.IsTrue(machine, condition) {
		logrus.Debugf("[plansecret] machine %s/%s: marking PlanApplied as true", machine.Namespace, machine.Name)
		conditions.MarkTrue(machine, condition)
		needsUpdate = true
	}

	if needsUpdate {
		logrus.Debugf("[plansecret] machine %s/%s: updating status of machine to reconcile for condition with error: %+v", machine.Namespace, machine.Name, planAppliedErr)
		_, err = h.machinesClient.UpdateStatus(machine)
	}

	return err
}
