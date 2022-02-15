package planner

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
)

type certificateRotation struct {
	rkeControlPlanes rkecontrollers.RKEControlPlaneClient
	store            *PlanStore
}

func newCertificateRotation(clients *wrangler.Context, store *PlanStore) *certificateRotation {
	return &certificateRotation{
		rkeControlPlanes: clients.RKE.RKEControlPlane(),
		store:            store,
	}
}

// RotateCertificates checks if there is a need to rotate any certificates and updates the plan accordingly.
func (r *certificateRotation) RotateCertificates(controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) error {
	if !shouldRotate(controlPlane) {
		return nil
	}

	rotatePlan := rotateCertificatesPlan(controlPlane, controlPlane.Spec.RotateCertificates)
	for _, node := range collect(clusterPlan, isControlPlaneEtcd) {
		err := r.store.UpdatePlan(node, rotatePlan, 0)
		if err != nil {
			return err
		}
	}

	controlPlane.Status.CertificateRotationGeneration = controlPlane.Spec.RotateCertificates.Generation
	_, err := r.rkeControlPlanes.UpdateStatus(controlPlane)
	return err
}

// shouldRotate `true` if any services are specified or the CA cert is to be rotated.
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
	if cp.Status.CertificateRotationGeneration == cp.Spec.RotateCertificates.Generation {
		return false
	}

	// atleast one service or the ca must be rotated for there to be any work
	if len(cp.Spec.RotateCertificates.Services) > 0 || cp.Spec.RotateCertificates.CACertificates {
		return true
	}

	return false
}

// rotateCertificatesPlan rotates the certificates for the services specified, if any, and restarts the service.  If `CACertificates` is true
// all certificates are rotated.
func rotateCertificatesPlan(controlPlane *rkev1.RKEControlPlane, rotation *rkev1.RotateCertificates) plan.NodePlan {
	args := []string{
		"certificate",
		"rotate",
	}

	if len(rotation.Services) > 0 && !rotation.CACertificates {
		for _, service := range rotation.Services {
			args = append(args, "-s", service)
		}
	}

	return plan.NodePlan{
		Instructions: []plan.OneTimeInstruction{
			{
				Name:    "stop service",
				Command: "systemctl",
				Args: []string{
					"stop",
					rke2.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion),
				},
			},
			{
				Name:    "rotate certificates",
				Command: rke2.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
				Args:    args,
			},
			{
				Name:    "start service",
				Command: "systemctl",
				Args: []string{
					"start",
					rke2.GetRuntimeServerUnit(controlPlane.Spec.KubernetesVersion),
				},
			},
		},
	}
}
