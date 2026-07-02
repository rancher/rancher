// Package cluster's caprke2.go provides helpers for the CAPRKE2 v2prov integration test.
//
// The test creates a CAPI Cluster whose control plane is a CAPRKE2 RKE2ControlPlane and whose
// infrastructure is a CAPI Docker DockerCluster. Turtles' rancher-auto-import annotation on the
// namespace then causes Rancher to create a corresponding management.cattle.io/v3 Cluster — but
// the operations the test exercises (etcd-snapshot save/restore, encryption-key-rotation) target
// the CAPI Cluster itself, because that's the GVK the CAPRKE2 adapter is registered for in
// pkg/operations/capi.go.
//
// These helpers are intentionally additive — no existing framework files are modified. The fixture
// object owns the created resources by parent namespace; deleting the namespace tears the whole
// cluster down.
package cluster

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2prov/clients"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// rancherAutoImportAnnotation is the annotation Turtles watches on namespaces to decide whether to
// mirror CAPI clusters in that namespace into Rancher as management.cattle.io/v3 Clusters.
// Setting this on the namespace is sufficient — no per-cluster opt-in is needed.
const rancherAutoImportAnnotation = "cluster-api.cattle.io/rancher-auto-import"

// CAPRKE2Provider GVKs used to build the cluster. Pulled into constants so test code doesn't
// scatter string literals.
var (
	gvkCluster               = schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster"}
	gvkDockerCluster         = schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "DockerCluster"}
	gvkDockerMachineTemplate = schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "DockerMachineTemplate"}
	gvkRKE2ControlPlane      = schema.GroupVersionKind{Group: "controlplane.cluster.x-k8s.io", Version: "v1beta2", Kind: "RKE2ControlPlane"}
	gvkMgmtV3Cluster         = schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "Cluster"}
)

// CAPRKE2Options controls cluster shape. All fields have sensible defaults; tests typically only
// override RKE2Version when pinning to a specific release for reproducibility.
type CAPRKE2Options struct {
	// NamePrefix is appended with a random suffix to form the cluster/control-plane/infra names.
	// Default: "caprke2".
	NamePrefix string
	// Namespace is the namespace that owns every created object. When empty a fresh random
	// namespace is created (annotated for Turtles auto-import).
	Namespace string
	// RKE2Version is the RKE2 release used by the control plane. Must match the
	// `vX.YY.Z+rke2rN` pattern enforced by CAPRKE2's webhook. Default: a known-good value baked
	// in to the helper (see defaultRKE2Version).
	RKE2Version string
	// Replicas is the RKE2ControlPlane replica count. Default: 1.
	Replicas int32
}

// defaultRKE2Version is the RKE2 release used when the test does not pin a specific one. Bump
// when CAPRKE2's webhook starts rejecting it. The trailing "+rke2r1" is required by the
// `(v\d\.\d{2}\.\d+\+rke2r\d)` pattern.
const defaultRKE2Version = "v1.32.5+rke2r1"

// CAPRKE2Fixture is what the helpers return to the test. The test should
//   - call WaitForCAPRKE2Ready to block until CAPI + Turtles auto-import are settled,
//   - build operation ClusterRefs using CAPIClusterRef(),
//   - and clean up by deleting the Namespace (which cascade-deletes everything else).
type CAPRKE2Fixture struct {
	Namespace   string
	ClusterName string
	// MgmtClusterName is the management.cattle.io/v3 Cluster name once Turtles auto-imports the
	// CAPI cluster. Populated by WaitForCAPRKE2Ready; the empty string before then.
	MgmtClusterName string
}

// CAPIClusterRef returns the corev1.ObjectReference the operation controllers expect when
// targeting the CAPI Cluster — NOT the auto-imported management.cattle.io v3 Cluster. The
// CAPRKE2 adapter is registered for the CAPI Cluster GVK at pkg/operations/capi.go.
func (f *CAPRKE2Fixture) CAPIClusterRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: gvkCluster.GroupVersion().String(),
		Kind:       gvkCluster.Kind,
		Name:       f.ClusterName,
		Namespace:  f.Namespace,
	}
}

// NewCAPRKE2Cluster creates the namespace, DockerCluster, DockerMachineTemplate, RKE2ControlPlane,
// and CAPI Cluster in dependency order. Returns the fixture; the cluster is NOT yet ready — call
// WaitForCAPRKE2Ready next.
func NewCAPRKE2Cluster(cs *clients.Clients, opts CAPRKE2Options) (*CAPRKE2Fixture, error) {
	if opts.NamePrefix == "" {
		opts.NamePrefix = "caprke2"
	}
	if opts.RKE2Version == "" {
		opts.RKE2Version = os.Getenv("SOME_K8S_VERSION")
	}
	if opts.Replicas == 0 {
		opts.Replicas = 1
	}

	ns := opts.Namespace
	if ns == "" {
		// 5-char rand suffix matches the convention v2prov uses elsewhere (see namespace.Random).
		ns = fmt.Sprintf("%s-%s", opts.NamePrefix, strings.ToLower(utilrand.String(5)))
		if err := cs.Client.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Annotations: map[string]string{
					// Tells Turtles to mirror CAPI clusters in this namespace into Rancher as
					// management.cattle.io/v3 Clusters with ImportedConfig (no provisioning).
					rancherAutoImportAnnotation: "true",
				},
			},
		}); err != nil {
			return nil, fmt.Errorf("creating namespace %s: %w", ns, err)
		}
	}

	name := fmt.Sprintf("%s-%s", opts.NamePrefix, strings.ToLower(utilrand.String(5)))

	// 1) DockerCluster — the CAPI infrastructure. We rely on Docker's default networking; no
	//    failureDomain or LoadBalancer spec required for a single-host setup.
	dockerCluster := newUnstructured(gvkDockerCluster, ns, name, map[string]any{
		"spec": map[string]any{},
	})
	if err := cs.Client.Create(context.TODO(), dockerCluster); err != nil {
		return nil, fmt.Errorf("creating DockerCluster %s/%s: %w", ns, name, err)
	}

	// 2) DockerMachineTemplate — the per-machine infrastructure template referenced by the
	//    RKE2ControlPlane.machineTemplate.infrastructureRef. Empty spec is acceptable for the
	//    Docker provider; defaults give us a kindest/node image at the chosen RKE2 version.
	dockerMachineTemplate := newUnstructured(gvkDockerMachineTemplate, ns, name, map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{},
			},
		},
	})
	if err := cs.Client.Create(context.TODO(), dockerMachineTemplate); err != nil {
		return nil, fmt.Errorf("creating DockerMachineTemplate %s/%s: %w", ns, name, err)
	}

	// 3) RKE2ControlPlane — pins the RKE2 version, points at the DockerMachineTemplate.
	//    machineTemplate.spec.infrastructureRef is Required by the v1beta2 CRD; the reference lives
	//    under `.spec`, NOT directly under machineTemplate. rolloutStrategy is a non-nullable object
	//    on the CRD (no `+optional`), so we must supply a concrete value — RollingUpdate/maxSurge=1
	//    matches the controller default.
	rke2ControlPlane := newUnstructured(gvkRKE2ControlPlane, ns, name, map[string]any{
		"spec": map[string]any{
			"version":  opts.RKE2Version,
			"replicas": opts.Replicas,
			"machineTemplate": map[string]any{
				"spec": map[string]any{
					"infrastructureRef": map[string]any{
						"apiGroup": gvkDockerMachineTemplate.Group,
						"kind":     gvkDockerMachineTemplate.Kind,
						"name":     name,
					},
				},
			},
			"rolloutStrategy": map[string]any{
				"type": "RollingUpdate",
				"rollingUpdate": map[string]any{
					"maxSurge": 1,
				},
			},
			"serverConfig": map[string]any{
				// Default CNI on RKE2 is canal; keep it explicit so the adapter's Calico-probe
				// gating (which only fires on cni=calico) reads predictably.
				"cni": "canal",
			},
		},
	})
	if err := cs.Client.Create(context.TODO(), rke2ControlPlane); err != nil {
		return nil, fmt.Errorf("creating RKE2ControlPlane %s/%s: %w", ns, name, err)
	}

	// 4) CAPI Cluster — wires up infra + control-plane refs. Both refs use ContractVersionedObjectReference
	//    (apiGroup + kind, NO version); CAPI resolves the version via contract labels.
	capiCluster := newUnstructured(gvkCluster, ns, name, map[string]any{
		"spec": map[string]any{
			"infrastructureRef": map[string]any{
				"apiGroup": gvkDockerCluster.Group,
				"kind":     gvkDockerCluster.Kind,
				"name":     name,
			},
			"controlPlaneRef": map[string]any{
				"apiGroup": gvkRKE2ControlPlane.Group,
				"kind":     gvkRKE2ControlPlane.Kind,
				"name":     name,
			},
		},
	})
	if err := cs.Client.Create(context.TODO(), capiCluster); err != nil {
		return nil, fmt.Errorf("creating Cluster %s/%s: %w", ns, name, err)
	}

	return &CAPRKE2Fixture{Namespace: ns, ClusterName: name}, nil
}

// WaitForCAPRKE2Ready polls the CAPI Cluster until its control plane is initialized and ready,
// then polls for Turtles to produce a matching management.cattle.io/v3 Cluster and waits for that
// mgmt cluster to be Ready. Updates fx.MgmtClusterName on success. 30-minute timeout — Docker
// CAPI bring-up is dominated by image pulls and RKE2 install.
func WaitForCAPRKE2Ready(t *testing.T, cs *clients.Clients, fx *CAPRKE2Fixture) {
	t.Helper()

	// 1) CAPI Cluster: wait for status.initialization.controlPlaneInitialized=true AND
	//    status.initialization.infrastructureProvisioned=true. In CAPI v1beta2 the top-level
	//    status.controlPlaneReady / status.infrastructureReady booleans were replaced by these
	//    nested initialization fields (see ClusterInitializationStatus in
	//    sigs.k8s.io/cluster-api/api/core/v1beta2/cluster_types.go).
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		capiCluster := &unstructured.Unstructured{}
		capiCluster.SetGroupVersionKind(gvkCluster)
		if err := cs.Client.Get(ctx, client.ObjectKey{Namespace: fx.Namespace, Name: fx.ClusterName}, capiCluster); err != nil {
			return false, err
		}
		cpInit, _, _ := unstructured.NestedBool(capiCluster.Object, "status", "initialization", "controlPlaneInitialized")
		infraProv, _, _ := unstructured.NestedBool(capiCluster.Object, "status", "initialization", "infrastructureProvisioned")
		return cpInit && infraProv, nil
	})
	if err != nil {
		// Dump CAPI Cluster + the two objects it references (DockerCluster infra, RKE2ControlPlane)
		// before failing. In CI the cluster is torn down when the test process exits, so this is
		// often the only chance to see why the control plane never came up.
		dumpCAPRKE2ObjectsOnFailure(t, cs, fx)
		t.Fatalf("timed out waiting for CAPI Cluster %s/%s initialization (controlPlaneInitialized+infrastructureProvisioned): %v", fx.Namespace, fx.ClusterName, err)
	}
	t.Logf("CAPI Cluster %s/%s: control plane + infrastructure ready", fx.Namespace, fx.ClusterName)

	// 2) Turtles auto-import: poll for a management.cattle.io/v3 Cluster whose
	//    `clusterapi.cluster.x-k8s.io/owned-by` (or similar) annotation references our CAPI
	//    cluster. Turtles' actual auto-import label/annotation key has shifted across versions;
	//    rather than hardcode it, we list mgmt v3 clusters and match the one whose name encodes
	//    or annotation references our CAPI namespace+name pair, or fall back to a name match on
	//    the cluster (Turtles names the mgmt cluster after the CAPI cluster).
	err = utilwait.PollUntilContextTimeout(cs.Ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		mgmtClusterList := &unstructured.UnstructuredList{}
		mgmtClusterList.SetGroupVersionKind(gvkMgmtV3Cluster)
		if err := cs.Client.List(ctx, mgmtClusterList); err != nil {
			return false, err
		}
		for _, mc := range mgmtClusterList.Items {
			// Turtles names the mgmt v3 cluster with the CAPI namespace+name encoded as
			// annotations. Match either an annotation pair or a display-name suffix.
			anns := mc.GetAnnotations()
			if anns["cluster-api.cattle.io/capi-cluster-name"] == fx.ClusterName &&
				anns["cluster-api.cattle.io/capi-cluster-namespace"] == fx.Namespace {
				fx.MgmtClusterName = mc.GetName()
				return true, nil
			}
			// Fallback: display-name matches the CAPI cluster name. Less reliable but handles
			// older Turtles versions that did not set the capi-cluster-* annotations.
			displayName, _, _ := unstructured.NestedString(mc.Object, "spec", "displayName")
			if displayName == fx.ClusterName {
				fx.MgmtClusterName = mc.GetName()
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for Turtles to auto-import CAPI Cluster %s/%s into a management.cattle.io v3 Cluster: %v", fx.Namespace, fx.ClusterName, err)
	}
	t.Logf("Turtles auto-imported CAPI Cluster %s/%s as management.cattle.io v3 Cluster %s", fx.Namespace, fx.ClusterName, fx.MgmtClusterName)

	// 3) Mgmt v3 Cluster Ready=true.
	err = utilwait.PollUntilContextTimeout(cs.Ctx, 10*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
		mgmtCluster := &unstructured.Unstructured{}
		mgmtCluster.SetGroupVersionKind(gvkMgmtV3Cluster)
		if err := cs.Client.Get(ctx, client.ObjectKey{Name: fx.MgmtClusterName}, mgmtCluster); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		conds, found, err := unstructured.NestedSlice(mgmtCluster.Object, "status", "conditions")
		if err != nil || !found {
			return false, nil
		}
		for _, c := range conds {
			cond, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := cond["type"].(string); t != "Ready" {
				continue
			}
			if s, _ := cond["status"].(string); s == "True" {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for management.cattle.io v3 Cluster %s to reach Ready=True: %v", fx.MgmtClusterName, err)
	}
	t.Logf("management.cattle.io v3 Cluster %s is Ready", fx.MgmtClusterName)
}

// DownstreamClient builds a kubernetes.Interface against the CAPRKE2 cluster by reading the
// admin kubeconfig that the CAPI cluster controller writes to a `<cluster>-kubeconfig` Secret
// once the control plane is up. The returned client lets tests do downstream CRUD (e.g. read a
// ConfigMap after a restore) without shelling out to kubectl from the test runner. This is the
// CAPRKE2 analogue of the imported test's `execKubectl` closure.
//
// Errors if the kubeconfig secret is missing or unparseable — call after WaitForCAPRKE2Ready so
// the secret is guaranteed to be present.
func (f *CAPRKE2Fixture) DownstreamClient(cs *clients.Clients) (kubernetes.Interface, error) {
	secret, err := cs.Core.Secret().Get(f.Namespace, fmt.Sprintf("%s-kubeconfig", f.ClusterName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting %s-kubeconfig: %w", f.ClusterName, err)
	}
	data := secret.Data["value"]
	if len(data) == 0 {
		return nil, fmt.Errorf("kubeconfig secret %s/%s has no 'value' data key", f.Namespace, secret.Name)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parsing kubeconfig from %s/%s: %w", f.Namespace, secret.Name, err)
	}
	return kubernetes.NewForConfig(cfg)
}

func newUnstructured(gvk schema.GroupVersionKind, namespace, name string, body map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	if body != nil {
		// Merge body into u.Object so callers can supply nested spec/status maps without
		// re-stating GVK/namespace/name themselves.
		for k, v := range body {
			u.Object[k] = v
		}
	}
	return u
}

// dumpCAPRKE2ObjectsOnFailure emits YAML dumps of the CAPI Cluster, the DockerCluster
// (infrastructureRef target), and the RKE2ControlPlane (controlPlaneRef target) so a failed CI
// run has enough state to diagnose the timeout without live cluster access. All three share the
// same name/namespace by construction (see NewCAPRKE2Cluster).
func dumpCAPRKE2ObjectsOnFailure(t *testing.T, cs *clients.Clients, fx *CAPRKE2Fixture) {
	t.Helper()
	for _, target := range []struct {
		label string
		gvk   schema.GroupVersionKind
	}{
		{"CAPI Cluster", gvkCluster},
		{"DockerCluster (infrastructure)", gvkDockerCluster},
		{"RKE2ControlPlane (control plane)", gvkRKE2ControlPlane},
	} {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(target.gvk)
		if err := cs.Client.Get(cs.Ctx, client.ObjectKey{Namespace: fx.Namespace, Name: fx.ClusterName}, obj); err != nil {
			t.Logf("dump %s %s/%s: get failed: %v", target.label, fx.Namespace, fx.ClusterName, err)
			continue
		}
		out, err := yaml.Marshal(obj.Object)
		if err != nil {
			t.Logf("dump %s %s/%s: marshal failed: %v", target.label, fx.Namespace, fx.ClusterName, err)
			continue
		}
		t.Logf("dump %s %s/%s:\n%s", target.label, fx.Namespace, fx.ClusterName, string(out))
	}
}
