package planner

import (
	"encoding/base64"
	"fmt"
	"strconv"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
)

// rotateCertificates checks if there is a need to rotate any certificates and updates the plan accordingly.
func (p *Planner) rotateCertificates(controlPlane *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, clusterPlan *plan.Plan) (rkev1.RKEControlPlaneStatus, error) {
	if !shouldRotate(controlPlane) {
		return status, nil
	}

	if err := p.pauseCAPICluster(controlPlane, true); err != nil {
		return status, ErrWaiting("pausing CAPI cluster")
	}

	for _, node := range collect(clusterPlan, anyRole) {
		if !shouldRotateEntry(controlPlane.Spec.RotateCertificates, node) {
			continue
		}

		rotatePlan := rotateCertificatesPlan(controlPlane, controlPlane.Spec.RotateCertificates, node)
		err := assignAndCheckPlan(p.store, fmt.Sprintf("[%s] certificate rotation", node.Machine.Name), node, rotatePlan, 0, 0)
		if err != nil {
			return status, err
		}
	}

	if err := p.pauseCAPICluster(controlPlane, false); err != nil {
		return status, ErrWaiting("unpausing CAPI cluster")
	}

	status.CertificateRotationGeneration = controlPlane.Spec.RotateCertificates.Generation
	return status, ErrWaiting("certificate rotation done")
}

// shouldRotate `true` if the cluster is ready and the generation is stale
func shouldRotate(cp *rkev1.RKEControlPlane) bool {
	// The controlplane must be initialized before we rotate anything
	if cp.Status.Initialized != true {
		return false
	}

	// if a spec is not defined there is nothing to do
	if cp.Spec.RotateCertificates == nil {
		return false
	}

	// if this generation has already been applied there is no work
	return cp.Status.CertificateRotationGeneration != cp.Spec.RotateCertificates.Generation
}

const idempotentRotateScript = `
#!/bin/sh

currentGeneration=""
targetGeneration=$2
runtime=$1
shift
shift

dataRoot="/var/lib/rancher/$runtime/certificate_rotation"
generationFile="$dataRoot/generation"

currentGeneration=$(cat "$generationFile" || echo "")

if [ "$currentGeneration" != "$targetGeneration" ]; then
  $runtime certificate rotate  $@
else
	echo "certificates have already been rotated to the current generation."
fi

mkdir -p $dataRoot
echo $targetGeneration > "$generationFile"
`

// rotateCertificatesPlan rotates the certificates for the services specified, if any, and restarts the service.  If no services are specified
// all certificates are rotated.
func rotateCertificatesPlan(controlPlane *rkev1.RKEControlPlane, rotation *rkev1.RotateCertificates, entry *planEntry) plan.NodePlan {
	if isOnlyWorker(entry) {
		return plan.NodePlan{
			Instructions: []plan.OneTimeInstruction{
				{
					Name:    "restart",
					Command: "systemctl",
					Args: []string{
						"restart",
						rke2.GetRuntimeAgentUnit(controlPlane.Spec.KubernetesVersion),
					},
				},
			},
		}
	}

	rotateScriptPath := "/var/lib/rancher/" + rke2.GetRuntime(controlPlane.Spec.KubernetesVersion) + "/rancher_v2prov_certificate_rotation/bin/rotate.sh"

	args := []string{
		"-xe",
		rotateScriptPath,
		rke2.GetRuntime(controlPlane.Spec.KubernetesVersion),
		strconv.FormatInt(rotation.Generation, 10),
	}

	if len(rotation.Services) > 0 {
		for _, service := range rotation.Services {
			args = append(args, "-s", service)
		}
	}

	return plan.NodePlan{
		Files: []plan.File{
			{
				Content: base64.StdEncoding.EncodeToString([]byte(idempotentRotateScript)),
				Path:    rotateScriptPath,
			},
		},
		Instructions: []plan.OneTimeInstruction{
			{
				Name:    "rotate certificates",
				Command: "sh",
				Args:    args,
			},
			{
				Name:    "restart",
				Command: "systemctl",
				Args: []string{
					"restart",
					rke2.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion),
				},
			},
		},
	}
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
