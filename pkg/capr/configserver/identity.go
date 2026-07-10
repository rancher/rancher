package configserver

import (
	"fmt"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// CallerKind classifies a /v3/connect/agent caller by how its cluster is provisioned/imported.
type CallerKind int

const (
	// KindV2Prov is a Rancher v2 provisioned "custom" (unmanaged) cluster — the cluster token
	// lives in the provisioning cluster's namespace (e.g. fleet-default), not in a mgmt cluster
	// namespace. Lifecycle owners are provisioning.Cluster + CAPI Machine.
	KindV2Prov CallerKind = iota
	// KindImported is a Rancher-imported RKE2/K3s cluster (mgmt cluster + mgmt Node lifecycle).
	KindImported
	// KindCAPINative is a CAPI cluster imported into Rancher via turtles (CAPI Cluster +
	// CAPI Machine lifecycle). The mgmt cluster is a shell that back-references the real CAPI
	// Cluster via CAPIClusterOwnerLabel / CAPIClusterOwnerNSLabel.
	KindCAPINative
)

func (k CallerKind) String() string {
	switch k {
	case KindV2Prov:
		return "v2prov"
	case KindImported:
		return "imported"
	case KindCAPINative:
		return "capi-native"
	}
	return "unknown"
}

// LifecycleContext carries everything downstream code needs to build/find machine-plans and
// lifecycle-labeled resources for a caller, without any string parsing of namespace names.
// Not every field is populated for every Kind — see ResolveMgmtTokenCaller.
type LifecycleContext struct {
	Kind CallerKind

	// TargetNamespace is where machine-plan secrets, plan SAs, and RBAC live for this caller.
	// For KindImported this is the mgmt cluster's namespace (mgmtCluster.Name); for KindCAPINative
	// it is the CAPI cluster's namespace.
	TargetNamespace string

	// MgmtCluster is always set — the mgmt-cluster obj that the caller's cluster token dereferenced
	// to (a real one for KindImported, a turtles-created shell for KindCAPINative).
	MgmtCluster *apimgmtv3.Cluster

	// CAPICluster is set only for KindCAPINative.
	CAPICluster *capi.Cluster
}

// ResolveMgmtTokenCaller classifies a caller that authenticated with a ClusterRegistrationToken.
// It dereferences the token's namespace to a mgmtv3.Cluster (if one exists) and inspects turtles
// back-reference labels to decide between KindImported and KindCAPINative. Namespaces that do not
// correspond to a mgmt cluster (e.g. fleet-default) fall through to KindV2Prov — the traditional
// unmanaged/custom-node path.
func ResolveMgmtTokenCaller(
	mgmtClusterCache mgmtcontroller.ClusterCache,
	capiClusterCache capicontrollers.ClusterCache,
	tokenNamespace string,
) (*LifecycleContext, error) {
	mgmtCluster, err := mgmtClusterCache.Get(tokenNamespace)
	if apierrors.IsNotFound(err) {
		return &LifecycleContext{
			Kind:            KindV2Prov,
			TargetNamespace: tokenNamespace,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	// v2prov clusters (custom + node-driver) have a mgmt cluster *shell* stamped with the
	// `provisioning.cattle.io/administrated=true` annotation. The real state lives on the
	// provisioning.cattle.io/Cluster, and downstream handlers (getCAPICluster / createMachine)
	// navigate mgmt shell → provv1.Cluster → CAPI cluster in fleet-default. Classify as
	// KindV2Prov so onSecretChange falls through to that path rather than mistakenly running the
	// imported RKE2/K3s (mgmt v3 Node) handler.
	if mgmtCluster.Annotations["provisioning.cattle.io/administrated"] == "true" {
		return &LifecycleContext{
			Kind:            KindV2Prov,
			TargetNamespace: tokenNamespace,
			MgmtCluster:     mgmtCluster,
		}, nil
	}

	ownerName := mgmtCluster.Labels[capr.CAPIClusterOwnerLabel]
	ownerNS := mgmtCluster.Labels[capr.CAPIClusterOwnerNSLabel]

	// Both labels must be set together — mixed state is misconfiguration and we refuse to
	// silently default to "imported", which has historically been the case with the prior
	// regex-based failure mode.
	if (ownerName == "") != (ownerNS == "") {
		return nil, fmt.Errorf(
			"mgmt cluster %s carries only one of %s/%s; both must be set for a CAPI-native cluster",
			mgmtCluster.Name, capr.CAPIClusterOwnerLabel, capr.CAPIClusterOwnerNSLabel)
	}

	if ownerName == "" {
		return &LifecycleContext{
			Kind:            KindImported,
			TargetNamespace: mgmtCluster.Name,
			MgmtCluster:     mgmtCluster,
		}, nil
	}

	capiCluster, err := capiClusterCache.Get(ownerNS, ownerName)
	if apierrors.IsNotFound(err) {
		return nil, fmt.Errorf(
			"mgmt cluster %s references CAPI cluster %s/%s, but that cluster was not found",
			mgmtCluster.Name, ownerNS, ownerName)
	}
	if err != nil {
		return nil, err
	}

	return &LifecycleContext{
		Kind:            KindCAPINative,
		TargetNamespace: capiCluster.Namespace,
		MgmtCluster:     mgmtCluster,
		CAPICluster:     capiCluster,
	}, nil
}

// findCAPIMachineByID looks up the CAPI Machine in the given namespace whose capr.MachineIDLabel
// matches machineID and whose capi.ClusterNameLabel matches clusterName. Returns (nil, nil) when
// no machine matches — callers retry / wait, they do not synthesize objects.
func findCAPIMachineByID(
	machineCache capicontrollers.MachineCache,
	namespace, clusterName, machineID string,
) (*capi.Machine, error) {
	if machineID == "" {
		return nil, nil
	}
	ms, err := machineCache.List(namespace, labels.SelectorFromSet(labels.Set{
		capr.MachineIDLabel:   machineID,
		capi.ClusterNameLabel: clusterName,
	}))
	if err != nil {
		return nil, err
	}
	if len(ms) == 0 {
		return nil, nil
	}
	if len(ms) > 1 {
		return nil, fmt.Errorf(
			"expected at most one CAPI Machine with %s=%s and %s=%s in %s, found %d",
			capr.MachineIDLabel, machineID, capi.ClusterNameLabel, clusterName, namespace, len(ms))
	}
	return ms[0], nil
}
