package planner

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/sirupsen/logrus"
)

// rotateCertificates checks if there is a need to rotate any certificates and updates the plan accordingly.
func (p *Planner) rotateCertificates(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	if !shouldRotate(controlPlane) {
		return status, nil
	}

	found, joinServer, _, err := p.findInitNode(controlPlane, clusterPlan)
	if err != nil {
		logrus.Errorf("[planner] rkecluster %s/%s: error encountered while searching for init node during certificate rotation: %v", controlPlane.Namespace, controlPlane.Name, err)
		return status, err
	}
	if !found || joinServer == "" {
		logrus.Warnf("[planner] rkecluster %s/%s: skipping certificate creation as cluster does not have an init node", controlPlane.Namespace, controlPlane.Name)
		return status, nil
	}

	// Assemble our list of nodes in order of etcd-only, etcd with controlplane, controlplane-only, and everything else
	orderedEntriesToRotate := collectOrderedCertificateRotationEntries(clusterPlan)

	for _, node := range orderedEntriesToRotate {
		if !shouldRotateEntry(controlPlane.Spec.RotateCertificates, node) {
			continue
		}

		rotatePlan, joinedServer, err := p.rotateCertificatesPlan(controlPlane, tokensSecret, controlPlane.Spec.RotateCertificates, node, joinServer)
		if err != nil {
			return status, err
		}

		err = assignAndCheckPlan(p.store, fmt.Sprintf("[%s] certificate rotation", node.Machine.Name), node, rotatePlan, joinedServer, 0, 0)
		if err != nil {
			// Ensure the CAPI cluster is paused if we have assigned and are checking a plan.
			if pauseErr := p.pauseCAPICluster(controlPlane, true); pauseErr != nil {
				return status, pauseErr
			}
			return status, err
		}
	}

	if err := p.pauseCAPICluster(controlPlane, false); err != nil {
		return status, errWaiting("unpausing CAPI cluster")
	}

	status.CertificateRotationGeneration = controlPlane.Spec.RotateCertificates.Generation
	return status, errWaiting("certificate rotation done")
}

func collectOrderedCertificateRotationEntries(clusterPlan *plan.Plan) []*planEntry {
	orderedEntriesToRotate := collect(clusterPlan, IsOnlyEtcd)                                                        // etcd or etcd + worker
	orderedEntriesToRotate = append(orderedEntriesToRotate, collect(clusterPlan, roleAnd(isControlPlane, isEtcd))...) // etcd + controlplane or etcd + controlplane+worker
	orderedEntriesToRotate = append(orderedEntriesToRotate, collect(clusterPlan, isOnlyControlPlane)...)              // controlplane or controlplane + worker
	orderedEntriesToRotate = append(orderedEntriesToRotate, collect(clusterPlan, isOnlyWorker)...)                    // worker
	return orderedEntriesToRotate
}

// shouldRotate `true` if the cluster is ready and the generation is stale
func shouldRotate(cp *rkev1.RKEControlPlane) bool {
	// if a spec is not defined there is nothing to do
	if cp.Spec.RotateCertificates == nil {
		return false
	}

	// The controlplane must be initialized before we rotate anything
	if cp.Status.Initialized != true {
		logrus.Warnf("[planner] rkecluster %s/%s: skipping certificate rotation as cluster was not initialized", cp.Namespace, cp.Name)
		return false
	}

	// if this generation has already been applied there is no work
	return cp.Status.CertificateRotationGeneration != cp.Spec.RotateCertificates.Generation
}

// rotateCertificatesPlan rotates the certificates for the services specified, if any, and restarts the service.  If no services are specified
// all certificates are rotated.
func (p *Planner) rotateCertificatesPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, rotation *rkev1.RotateCertificates, entry *planEntry, joinServer string) (plan.NodePlan, string, error) {
	if isOnlyWorker(entry) {
		// Don't overwrite the joinURL annotation.
		joinServer = ""
	}
	rotatePlan, config, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, entry, joinServer, true)
	if err != nil {
		return plan.NodePlan{}, joinedServer, err
	}

	if isOnlyWorker(entry) {
		rotatePlan.Instructions = append(rotatePlan.Instructions, idempotentRestartInstructions(
			controlPlane,
			"certificate-rotation/restart",
			strconv.FormatInt(rotation.Generation, 10),
			capr.GetRuntimeAgentUnit(controlPlane.Spec.KubernetesVersion))...)
		return rotatePlan, joinedServer, nil
	}

	rotatePlan.Instructions = append(rotatePlan.Instructions, idempotentStopInstruction(
		controlPlane,
		"certificate-rotation/stop",
		strconv.FormatInt(rotation.Generation, 10),
		capr.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion)))

	args := []string{
		"certificate",
		"rotate",
	}

	if len(rotation.Services) > 0 {
		for _, service := range rotation.Services {
			args = append(args, "-s", service)
		}
	}

	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)

	rotatePlan.Instructions = append(rotatePlan.Instructions, idempotentInstruction(
		controlPlane,
		"certificate-rotation/rotate",
		strconv.FormatInt(rotation.Generation, 10),
		capr.GetRuntime(controlPlane.Spec.KubernetesVersion),
		args,
		[]string{},
	))
	if isControlPlane(entry) {
		// The following kube-scheduler and kube-controller-manager certificates are self-signed by the respective services and are used by CAPR for secure healthz probes against the service.
		if rotationContainsService(rotation, "controller-manager") {
			if kcmCertDir := getArgValue(config[KubeControllerManagerArg], CertDirArgument, "="); kcmCertDir != "" && getArgValue(config[KubeControllerManagerArg], TLSCertFileArgument, "=") == "" {
				rotatePlan.Instructions = append(rotatePlan.Instructions, []plan.OneTimeInstruction{
					idempotentInstruction(
						controlPlane,
						"certificate-rotation/rm-kcm-cert",
						strconv.FormatInt(rotation.Generation, 10),
						"rm",
						[]string{
							"-f",
							fmt.Sprintf("%s/%s", kcmCertDir, DefaultKubeControllerManagerCert),
						},
						[]string{},
					),
					idempotentInstruction(
						controlPlane,
						"certificate-rotation/rm-kcm-key",
						strconv.FormatInt(rotation.Generation, 10),
						"rm",
						[]string{
							"-f",
							fmt.Sprintf("%s/%s", kcmCertDir, strings.ReplaceAll(DefaultKubeControllerManagerCert, ".crt", ".key")),
						},
						[]string{},
					),
				}...)
				if runtime == capr.RuntimeRKE2 {
					rotatePlan.Instructions = append(rotatePlan.Instructions, idempotentInstruction(
						controlPlane,
						"certificate-rotation/rm-kcm-spm",
						strconv.FormatInt(rotation.Generation, 10),
						"rm",
						[]string{
							"-f",
							path.Join(capr.GetDistroDataDir(controlPlane), "/agent/pod-manifests/kube-controller-manager.yaml"),
						},
						[]string{},
					))
				}
			}
		}
		if rotationContainsService(rotation, "scheduler") {
			if ksCertDir := getArgValue(config[KubeSchedulerArg], CertDirArgument, "="); ksCertDir != "" && getArgValue(config[KubeSchedulerArg], TLSCertFileArgument, "=") == "" {
				rotatePlan.Instructions = append(rotatePlan.Instructions, []plan.OneTimeInstruction{
					idempotentInstruction(
						controlPlane,
						"certificate-rotation/rm-ks-cert",
						strconv.FormatInt(rotation.Generation, 10),
						"rm",
						[]string{
							"-f",
							fmt.Sprintf("%s/%s", ksCertDir, DefaultKubeSchedulerCert),
						},
						[]string{},
					),
					idempotentInstruction(
						controlPlane,
						"certificate-rotation/rm-ks-key",
						strconv.FormatInt(rotation.Generation, 10),
						"rm",
						[]string{
							"-f",
							fmt.Sprintf("%s/%s", ksCertDir, strings.ReplaceAll(DefaultKubeSchedulerCert, ".crt", ".key")),
						},
						[]string{},
					),
				}...)
				if runtime == capr.RuntimeRKE2 {
					rotatePlan.Instructions = append(rotatePlan.Instructions, idempotentInstruction(
						controlPlane,
						"certificate-rotation/rm-ks-spm",
						strconv.FormatInt(rotation.Generation, 10),
						"rm",
						[]string{
							"-f",
							path.Join(capr.GetDistroDataDir(controlPlane), "agent/pod-manifests/kube-scheduler.yaml"),
						},
						[]string{},
					))
				}
			}
		}
	}
	if runtime == capr.RuntimeRKE2 {
		if generated, instruction := generateManifestRemovalInstruction(controlPlane, entry); generated {
			rotatePlan.Instructions = append(rotatePlan.Instructions, convertToIdempotentInstruction(
				controlPlane,
				"certificate-rotation/manifest-removal",
				strconv.FormatInt(rotation.Generation, 10),
				instruction))
		}
	}
	rotatePlan.Instructions = append(rotatePlan.Instructions, idempotentRestartInstructions(
		controlPlane,
		"certificate-rotation/restart",
		strconv.FormatInt(rotation.Generation, 10),
		capr.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion))...)
	return rotatePlan, joinedServer, nil
}

// rotationContainsService searches the rotation.Services slice the specified service. If the length of the services slice is 0, it returns true.
func rotationContainsService(rotation *rkev1.RotateCertificates, service string) bool {
	if rotation == nil {
		return false
	}
	if len(rotation.Services) == 0 {
		return true
	}
	for _, desiredService := range rotation.Services {
		if desiredService == service {
			return true
		}
	}
	return false
}

// shouldRotateEntry returns true if the rotated services are applicable to the entry's roles.
func shouldRotateEntry(rotation *rkev1.RotateCertificates, entry *planEntry) bool {
	relevantServices := map[string]struct{}{}

	if len(rotation.Services) == 0 {
		return true
	}

	if isWorker(entry) {
		relevantServices["rke2-server"] = struct{}{}
		relevantServices["k3s-server"] = struct{}{}
		relevantServices["api-server"] = struct{}{}
		relevantServices["kubelet"] = struct{}{}
		relevantServices["kube-proxy"] = struct{}{}
		relevantServices["auth-proxy"] = struct{}{}
	}

	if isControlPlane(entry) {
		relevantServices["rke2-server"] = struct{}{}
		relevantServices["k3s-server"] = struct{}{}
		relevantServices["api-server"] = struct{}{}
		relevantServices["kubelet"] = struct{}{}
		relevantServices["kube-proxy"] = struct{}{}
		relevantServices["auth-proxy"] = struct{}{}
		relevantServices["controller-manager"] = struct{}{}
		relevantServices["scheduler"] = struct{}{}
		relevantServices["rke2-controller"] = struct{}{}
		relevantServices["k3s-controller"] = struct{}{}
		relevantServices["admin"] = struct{}{}
		relevantServices["cloud-controller"] = struct{}{}
	}

	if isEtcd(entry) {
		relevantServices["etcd"] = struct{}{}
		relevantServices["kubelet"] = struct{}{}
		relevantServices["k3s-server"] = struct{}{}
		relevantServices["rke2-server"] = struct{}{}
	}

	for i := range rotation.Services {
		if _, ok := relevantServices[rotation.Services[i]]; ok {
			return true
		}
	}

	return false
}
