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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2prov/clients"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// rancherAutoImportLabel is the LABEL (not annotation — Turtles reads this from
// obj.GetLabels() in util.ShouldImport, see rancher/turtles/util package) Turtles watches on
// namespaces (or CAPI Clusters) to decide whether to mirror CAPI clusters in that namespace
// into Rancher as management.cattle.io/v3 Clusters. Setting this on the namespace is
// sufficient — no per-cluster opt-in needed.
const rancherAutoImportLabel = "cluster-api.cattle.io/rancher-auto-import"

// CAPRKE2Provider GVKs used to build the cluster. Pulled into constants so test code doesn't
// scatter string literals.
var (
	gvkCluster               = schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster"}
	gvkDockerCluster         = schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "DockerCluster"}
	gvkDockerMachineTemplate = schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "DockerMachineTemplate"}
	gvkRKE2ControlPlane      = schema.GroupVersionKind{Group: "controlplane.cluster.x-k8s.io", Version: "v1beta2", Kind: "RKE2ControlPlane"}
	gvkRKE2ConfigTemplate    = schema.GroupVersionKind{Group: "bootstrap.cluster.x-k8s.io", Version: "v1beta2", Kind: "RKE2ConfigTemplate"}
	gvkMachineDeployment     = schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeployment"}
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
	// WorkerReplicas is the MachineDeployment replica count for agent (worker-only) nodes. When
	// 0 no MachineDeployment is created and the cluster is control-plane-only. Default: 0.
	WorkerReplicas int32
	// S3, when set, configures etcd snapshots to an S3-compatible object store instead of (well,
	// in addition to) machine-local disk. See CAPRKE2S3.
	S3 *CAPRKE2S3
	// UseSnapshotFileName controls which identifier the test's ETCDSnapshotRestore operation
	// carries in spec.snapshot.name. When true the test passes rkev1.ETCDSnapshot.SnapshotFile.Name
	// (the raw on-disk file name like `etcd-snapshot-<host>-<unix>`); when false it passes the
	// upstream ETCDSnapshot CR name (like `<cluster>-<safe-name>`). Single-server CAPRKE2 clusters
	// need the file-name form; multi-node clusters need the CR-name form. Default: false.
	UseSnapshotFileName bool
}

// CAPRKE2S3 points the cluster's etcd snapshots at an S3-compatible object store. It is rendered onto
// the RKE2ControlPlane as spec.serverConfig.etcd.backupConfig.s3, which CAPRKE2 turns into the
// etcd-s3* keys of each server's config.yaml (see cluster-api-provider-rke2/pkg/rke2/config.go).
//
// S3 is what makes a snapshot survive a control-plane roll, and — more importantly — what makes it
// survive with its extra metadata intact. RKE2 stamps ETCDSnapshotFile.Spec.Metadata only on the
// resource belonging to the node that took the snapshot; a node that merely re-discovers a local file
// registers it bare, because the metadata is not persisted next to the file. For S3 the metadata is
// uploaded alongside the snapshot (as <folder>/.metadata/<name>) and read back by whichever node
// lists the bucket, so the restore modes the snapshot captured are still on offer after the machine
// that took it is gone.
type CAPRKE2S3 struct {
	// Endpoint is host:port, reachable from inside the machine containers. objectstore's external
	// mode returns a suitable one; a ClusterIP will not work, since CAPD machines sit outside the
	// local cluster's pod network.
	Endpoint string
	// EndpointCA is the PEM CA bundle for Endpoint, base64-encoded — i.e. objectstore.Info.Cert
	// verbatim.
	EndpointCA string
	Bucket     string
	Folder     string
	AccessKey  string
	SecretKey  string
}

// s3SecretNames returns the names of the two Secrets CAPRKE2 reads the S3 configuration from. Keyed
// off the cluster name so parallel tests in one namespace do not collide.
func s3SecretNames(clusterName string) (credentials, endpointCA string) {
	return clusterName + "-s3-credentials", clusterName + "-s3-ca"
}

// createS3Secrets creates the credential and CA Secrets in the cluster's namespace. The key names are
// fixed by CAPRKE2: "aws_access_key_id"/"aws_secret_access_key" for credentials and "ca.pem" for the
// CA, which it writes to /etc/rancher/rke2/etcd-s3-ca.crt on each machine.
func createS3Secrets(cs *clients.Clients, ns, clusterName string, s3 *CAPRKE2S3) error {
	credentialsName, endpointCAName := s3SecretNames(clusterName)

	if err := cs.Client.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: credentialsName},
		Data: map[string][]byte{
			"aws_access_key_id":     []byte(s3.AccessKey),
			"aws_secret_access_key": []byte(s3.SecretKey),
		},
	}); err != nil {
		return fmt.Errorf("creating S3 credential Secret %s/%s: %w", ns, credentialsName, err)
	}

	ca, err := base64.StdEncoding.DecodeString(s3.EndpointCA)
	if err != nil {
		return fmt.Errorf("decoding S3 endpoint CA: %w", err)
	}

	if err := cs.Client.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: endpointCAName},
		Data:       map[string][]byte{"ca.pem": ca},
	}); err != nil {
		return fmt.Errorf("creating S3 endpoint CA Secret %s/%s: %w", ns, endpointCAName, err)
	}

	return nil
}

// etcdBackupConfig renders spec.serverConfig.etcd for the given S3 settings, or nil when the cluster
// keeps snapshots on machine-local disk only.
//
// enforceSslVerify is deliberately true: the object store's certificate covers every node IP (see
// objectstore.GetExternalObjectStore), so there is no reason to weaken this, and CAPRKE2 inverts the
// field into etcd-s3-skip-ssl-verify.
func etcdBackupConfig(ns, clusterName string, s3 *CAPRKE2S3) map[string]any {
	if s3 == nil {
		return nil
	}

	credentialsName, endpointCAName := s3SecretNames(clusterName)

	return map[string]any{
		"backupConfig": map[string]any{
			"s3": map[string]any{
				"endpoint":         s3.Endpoint,
				"bucket":           s3.Bucket,
				"folder":           s3.Folder,
				"enforceSslVerify": true,
				"s3CredentialSecret": map[string]any{
					"name":      credentialsName,
					"namespace": ns,
				},
				"endpointCAsecret": map[string]any{
					"name":      endpointCAName,
					"namespace": ns,
				},
			},
		},
	}
}

// defaultRKE2Version is the RKE2 release used when the test does not pin a specific one. Bump
// when CAPRKE2's webhook starts rejecting it. The trailing "+rke2r1" is required by the
// `(v\d\.\d{2}\.\d+\+rke2r\d)` pattern.
const defaultRKE2Version = "v1.32.5+rke2r1"

// defaultKindestNodeImage is what CAPD launches for each control-plane machine. Without this,
// CAPD tries to derive a tag from the RKE2 version — e.g. `kindest/node:v1.32.5_rke2r1` — which
// is not a published image (kindest publishes plain-K8s tags like `kindest/node:v1.34.0`). The
// image is only used as the systemd container base; RKE2 installs its own kubelet on top, so the
// K8s version encoded in the tag does not have to match RKE2's. Matches CAPRKE2's own upstream
// examples (see cluster-api-provider-rke2/examples/clusterclass/docker/clusterclass-template.yaml).
const defaultKindestNodeImage = "kindest/node:v1.34.0"

// CAPRKE2Fixture is what the helpers return to the test. The test should
//   - call WaitForCAPRKE2Ready to block until CAPI + Turtles auto-import are settled,
//   - build operation ClusterRefs using CAPIClusterRef(),
//   - and clean up by deleting the Namespace (which cascade-deletes everything else).
type CAPRKE2Fixture struct {
	Namespace   string
	ClusterName string
	// WorkerMachineDeploymentName is the name of the worker MachineDeployment (empty when the
	// cluster was created with WorkerReplicas == 0). Used by WaitForCAPRKE2Ready to decide
	// whether to wait for a MachineDeployment to become ready.
	WorkerMachineDeploymentName string
	// WorkerReplicas mirrors the CAPRKE2Options value so WaitForCAPRKE2Ready can assert the
	// expected number of ready worker machines.
	WorkerReplicas int32
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

// MgmtClusterRef returns the corev1.ObjectReference for the management.cattle.io v3 Cluster that
// turtles auto-imported for this CAPI Cluster. Populated by WaitForCAPRKE2Ready; call it after.
//
// Prefer this over CAPIClusterRef for any operation that needs the rkev1.ETCDSnapshot resource.
// Addressing the mgmt Cluster routes adapter construction through pkg/operations/imported.go's
// factory, which threads the mgmt cluster name into the CAPRKE2 adapter so
// EtcdSnapshotNamespace() resolves to the mgmt cluster's namespace — where snapshotbackpopulate
// actually writes the snapshot CRs. Addressing the CAPI Cluster directly leaves that name empty and
// the adapter falls back to the CAPI namespace, where no snapshot CR exists. It is also the path the
// Rancher UI uses (see the comment on the factory in pkg/operations/imported.go).
//
// mgmt v3 Clusters are cluster-scoped, so the reference carries no namespace.
func (f *CAPRKE2Fixture) MgmtClusterRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: gvkMgmtV3Cluster.GroupVersion().String(),
		Kind:       gvkMgmtV3Cluster.Kind,
		Name:       f.MgmtClusterName,
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
				Labels: map[string]string{
					// Tells Turtles to mirror CAPI clusters in this namespace into Rancher as
					// management.cattle.io/v3 Clusters with ImportedConfig (no provisioning).
					// This MUST be a label, not an annotation: Turtles' import controller
					// (rancher/turtles/util.ShouldImport) reads obj.GetLabels()[key].
					rancherAutoImportLabel: "true",
				},
			},
		}); err != nil {
			return nil, fmt.Errorf("creating namespace %s: %w", ns, err)
		}
	}

	name := fmt.Sprintf("%s-%s", opts.NamePrefix, strings.ToLower(utilrand.String(5)))

	// 0) Custom LB haproxy template — the stock CAPD LB only proxies the kube-apiserver port
	//    (6443). For RKE2 the agent (worker) join needs the RKE2 supervisor on 9345 too, and
	//    the agent's bootstrap config points at the LB's IP because that is what CAPI sets as
	//    Cluster.spec.controlPlaneEndpoint. Without this ConfigMap, worker joins fail with
	//    `connection refused` to <lb-ip>:9345. Content mirrors CAPRKE2's own upstream example
	//    (see cluster-api-provider-rke2/examples/templates/docker/cluster-template.yaml).
	lbCMName := name + "-lb-config"
	if err := cs.Client.Create(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: lbCMName},
		Data:       map[string]string{"value": caprke2LBConfigTemplate},
	}); err != nil {
		return nil, fmt.Errorf("creating LB config ConfigMap %s/%s: %w", ns, lbCMName, err)
	}

	// 1) DockerCluster — the CAPI infrastructure. Reference the custom LB template so the
	//    kindest/haproxy container proxies both kube-apiserver and the RKE2 supervisor.
	dockerCluster := newUnstructured(gvkDockerCluster, ns, name, map[string]any{
		"spec": map[string]any{
			"loadBalancer": map[string]any{
				"customHAProxyConfigTemplateRef": map[string]any{
					"name": lbCMName,
				},
			},
		},
	})
	if err := cs.Client.Create(context.TODO(), dockerCluster); err != nil {
		return nil, fmt.Errorf("creating DockerCluster %s/%s: %w", ns, name, err)
	}

	// 2) DockerMachineTemplate — the per-machine infrastructure template referenced by the
	//    RKE2ControlPlane.machineTemplate.infrastructureRef. customImage is set explicitly to a
	//    published kindest/node tag; without it CAPD derives a tag from the RKE2 version and
	//    ImagePull fails (kindest doesn't publish `_rke2rN` variants).
	dockerMachineTemplate := newUnstructured(gvkDockerMachineTemplate, ns, name, map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"customImage": defaultKindestNodeImage,
				},
			},
		},
	})
	if err := cs.Client.Create(context.TODO(), dockerMachineTemplate); err != nil {
		return nil, fmt.Errorf("creating DockerMachineTemplate %s/%s: %w", ns, name, err)
	}

	if opts.S3 != nil {
		if err := createS3Secrets(cs, ns, name, opts.S3); err != nil {
			return nil, err
		}
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
				// Disable RKE2's built-in stub cloud-controller-manager. That component sets
				// spec.providerID on every Node to `rke2://<name>` as soon as the node registers.
				// CAPD then tries to set its own `docker://…` providerID on the same Node and the
				// kube-apiserver rejects the patch — `spec.providerID` is one-shot immutable
				// ("Forbidden: node updates may not change providerID except from '' to valid").
				// Without a providerID, DockerMachine.spec.providerID never populates, Machine's
				// NodeHealthy stays Unknown, and Cluster.Available never flips to True.
				//
				// CAPRKE2 maps `disableComponents.kubernetesComponents: [cloudController]` to
				// `rke2 --disable-cloud-controller` (see cluster-api-provider-rke2/pkg/rke2/
				// config.go). No effect on Docker-backed clusters beyond letting CAPD own the
				// providerID slot.
				"disableComponents": map[string]any{
					"kubernetesComponents": []any{"cloudController"},
				},
				// kube-apiserver args pushed through to the workload cluster.
				//
				// anonymous-auth=true undoes RKE2's hardened default (--anonymous-auth=false, part of
				// its CIS-benchmark defaults) so that /healthz responds 200 unauthenticated. CAPD wires
				// a kindest/haproxy LB in front of every DockerMachine, and that image's baked-in
				// haproxy.cfg does `option httpchk GET /healthz` on the backend. With the RKE2 default,
				// /healthz returns 401, the backend is marked DOWN, and every client that reads the
				// workload kubeconfig (which points at the LB) sees TLS EOF — CAPRKE2's control-plane
				// controller then loops forever on "connection to the workload cluster is down" and
				// the RKE2ControlPlane never transitions to Initialized=True.
				//
				// This is a throwaway Docker-backed test cluster with no security posture to preserve,
				// so opening /healthz to anon is the right trade-off. If we ever need the hardened
				// default back, the alternative is a `spec.loadBalancer.customHAProxyConfigTemplateRef`
				// on the DockerCluster pointing at a template that uses `option tcp-check` instead.
				"kubeAPIServer": map[string]any{
					"extraArgs": []any{"anonymous-auth=true"},
				},
			},
		},
	})
	if etcd := etcdBackupConfig(ns, name, opts.S3); etcd != nil {
		if err := unstructured.SetNestedMap(rke2ControlPlane.Object, etcd, "spec", "serverConfig", "etcd"); err != nil {
			return nil, fmt.Errorf("setting etcd backup config on RKE2ControlPlane %s/%s: %w", ns, name, err)
		}
	}
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

	fx := &CAPRKE2Fixture{Namespace: ns, ClusterName: name, WorkerReplicas: opts.WorkerReplicas}

	if opts.WorkerReplicas > 0 {
		workerName := name + "-workers"

		// 5) Worker DockerMachineTemplate — same kindest/node base as the control-plane machines.
		//    A separate template (rather than reusing the CP one) keeps InfrastructureRef churn on
		//    the MachineDeployment independent of RKE2ControlPlane rollouts, matching CAPRKE2's
		//    upstream docker/cluster-template.yaml layout.
		workerDMT := newUnstructured(gvkDockerMachineTemplate, ns, workerName, map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"customImage": defaultKindestNodeImage,
					},
				},
			},
		})
		if err := cs.Client.Create(context.TODO(), workerDMT); err != nil {
			return nil, fmt.Errorf("creating worker DockerMachineTemplate %s/%s: %w", ns, workerName, err)
		}

		// 6) RKE2ConfigTemplate — agent-only bootstrap config. An empty agentConfig block is the
		//    CAPRKE2 idiom for "default agent"; the rke2 install script + join token are wired up
		//    by the CAPRKE2 bootstrap controller from the RKE2ControlPlane's server config.
		rke2ConfigTemplate := newUnstructured(gvkRKE2ConfigTemplate, ns, workerName, map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"agentConfig": map[string]any{},
					},
				},
			},
		})
		if err := cs.Client.Create(context.TODO(), rke2ConfigTemplate); err != nil {
			return nil, fmt.Errorf("creating RKE2ConfigTemplate %s/%s: %w", ns, workerName, err)
		}

		// 7) MachineDeployment — worker pool. Selector must match a label CAPI stamps on machine
		//    templates for this cluster (cluster.x-k8s.io/cluster-name), otherwise
		//    MachineDeployment.status.readyReplicas never converges.
		machineDeployment := newUnstructured(gvkMachineDeployment, ns, workerName, map[string]any{
			"spec": map[string]any{
				"clusterName": name,
				"replicas":    opts.WorkerReplicas,
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"cluster.x-k8s.io/cluster-name": name,
					},
				},
				"template": map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"cluster.x-k8s.io/cluster-name": name,
						},
					},
					"spec": map[string]any{
						"version":     opts.RKE2Version,
						"clusterName": name,
						"bootstrap": map[string]any{
							"configRef": map[string]any{
								"apiGroup": gvkRKE2ConfigTemplate.Group,
								"kind":     gvkRKE2ConfigTemplate.Kind,
								"name":     workerName,
							},
						},
						"infrastructureRef": map[string]any{
							"apiGroup": gvkDockerMachineTemplate.Group,
							"kind":     gvkDockerMachineTemplate.Kind,
							"name":     workerName,
						},
					},
				},
			},
		})
		if err := cs.Client.Create(context.TODO(), machineDeployment); err != nil {
			return nil, fmt.Errorf("creating MachineDeployment %s/%s: %w", ns, workerName, err)
		}
		fx.WorkerMachineDeploymentName = workerName
	}

	return fx, nil
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

	// 1b) Worker MachineDeployment (if any): wait for status.readyReplicas to match the desired
	//     replica count. The MachineDeployment is a plain CAPI object, so we can use the typed
	//     CAPI client directly.
	if fx.WorkerMachineDeploymentName != "" {
		err = utilwait.PollUntilContextTimeout(cs.Ctx, 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
			md, err := cs.CAPI.MachineDeployment().Get(fx.Namespace, fx.WorkerMachineDeploymentName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			if md.Status.ReadyReplicas == nil {
				return false, nil
			}
			return *md.Status.ReadyReplicas == fx.WorkerReplicas, nil
		})
		if err != nil {
			t.Fatalf("timed out waiting for worker MachineDeployment %s/%s to reach readyReplicas=%d: %v",
				fx.Namespace, fx.WorkerMachineDeploymentName, fx.WorkerReplicas, err)
		}
		t.Logf("worker MachineDeployment %s/%s: %d replicas ready", fx.Namespace, fx.WorkerMachineDeploymentName, fx.WorkerReplicas)
	}

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

// RunCAPRKE2Kubectl runs RKE2's local kubectl inside a CAPD control-plane container.
//
// Workload API access from the v2prov host depends on the test topology. Run the recovery check
// from the server instead, so it verifies the workload API rather than host network routing.
//
// This is intentionally limited to CAPRKE2Docker tests and requires the active Docker context.
func RunCAPRKE2Kubectl(ctx context.Context, machineName string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	command := []string{
		"exec",
		machineName,
		"/var/lib/rancher/rke2/bin/kubectl",
		"--kubeconfig=/etc/rancher/rke2/rke2.yaml",
		"--request-timeout=20s",
	}
	command = append(command, args...)

	cmd := exec.CommandContext(ctx, "docker", command...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("running kubectl in CAPRKE2 machine %s: %w: stdout=%q stderr=%q",
			machineName, err, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// DownstreamClient builds a kubernetes.Interface against the CAPRKE2 cluster by reading the
// admin kubeconfig that the CAPI cluster controller writes to a `<cluster>-kubeconfig` Secret
// once the control plane is up. The returned client lets tests do downstream CRUD (e.g. read a
// ConfigMap after a restore) without shelling out to kubectl from the test runner.
//
// Prefer RunCAPRKE2Kubectl for anything that only needs to run a command: it executes inside the
// control-plane container and so does not depend on the test host being able to route to the
// workload API. This client exists for the restore-mode tests, which read and compare typed objects
// (snapshot extra metadata, node kubelet versions) where driving kubectl through jsonpath would be
// materially harder to follow. They are local-dev only, so the host-routing caveat is acceptable
// there; think twice before reaching for it in a test that has to pass in CI.
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

// caprke2LBConfigTemplate is the haproxy config that the kindest/haproxy LB container renders.
// The CAPD LB manager (see cluster-api/test/infrastructure/docker/internal/loadbalancer) executes
// this as a Go text/template with `.FrontendControlPlanePort`, `.BackendControlPlanePort`,
// `.BackendServers`, `.IPv6`, and the `JoinHostPort` helper populated per-cluster.
//
// Content is a verbatim copy of CAPRKE2's own upstream Docker example:
// cluster-api-provider-rke2/examples/templates/docker/cluster-template.yaml. The critical bit
// (compared to the stock CAPD template) is the second frontend+backend pair — `rke2-join` on 9345
// proxying to `rke2-servers` — so agent nodes can reach the RKE2 supervisor via the LB IP that
// CAPI advertises as Cluster.spec.controlPlaneEndpoint.
const caprke2LBConfigTemplate = `# generated by kind
global
  log /dev/log local0
  log /dev/log local1 notice
  daemon
  # limit memory usage to approximately 18 MB
  # (see https://github.com/kubernetes-sigs/kind/pull/3115)
  maxconn 100000

resolvers docker
  nameserver dns 127.0.0.11:53

defaults
  log global
  mode tcp
  option dontlognull
  # TODO: tune these
  timeout connect 5000
  timeout client 50000
  timeout server 50000
  # allow to boot despite dns don't resolve backends
  default-server init-addr none

frontend stats
  mode http
  bind *:8404
  stats enable
  stats uri /stats
  stats refresh 1s
  stats admin if TRUE

frontend control-plane
  bind *:{{ .FrontendControlPlanePort }}
  {{ if .IPv6 -}}
  bind :::{{ .FrontendControlPlanePort }};
  {{- end }}
  default_backend kube-apiservers

backend kube-apiservers
  option httpchk GET /healthz

  {{range $server, $backend := .BackendServers}}
  server {{ $server }} {{ JoinHostPort $backend.Address $.BackendControlPlanePort }} check check-ssl verify none resolvers docker resolve-prefer {{ if $.IPv6 -}} ipv6 {{- else -}} ipv4 {{- end }}
  {{- end}}

frontend rke2-join
  bind *:9345
  {{ if .IPv6 -}}
  bind :::9345;
  {{- end }}
  default_backend rke2-servers

backend rke2-servers
  option httpchk GET /v1-rke2/readyz
  http-check expect status 403
  {{range $server, $backend := .BackendServers}}
  server {{ $server }} {{ $backend.Address }}:9345 check check-ssl verify none
  {{- end}}
`

// RKE2ControlPlane returns the cluster's RKE2ControlPlane as an unstructured object. It shares its
// name and namespace with the CAPI Cluster by construction (see NewCAPRKE2Cluster).
//
// The object is returned unstructured on purpose: CAPRKE2's CRDs are only installed when turtles is
// enabled, so the test suite has no generated typed client for them — the same reason
// pkg/operations/caprke2.go reaches for the dynamic client.
func (f *CAPRKE2Fixture) RKE2ControlPlane(cs *clients.Clients) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvkRKE2ControlPlane)
	if err := cs.Client.Get(cs.Ctx, client.ObjectKey{Namespace: f.Namespace, Name: f.ClusterName}, obj); err != nil {
		return nil, fmt.Errorf("getting RKE2ControlPlane %s/%s: %w", f.Namespace, f.ClusterName, err)
	}
	return obj, nil
}

// RKE2ControlPlaneVersion returns spec.version, the Kubernetes version the control plane is
// configured for. This is the field a kubernetesVersion restore rewrites — it is the only entry
// restoremode.WritablePaths permits for an RKE2ControlPlane.
func (f *CAPRKE2Fixture) RKE2ControlPlaneVersion(cs *clients.Clients) (string, error) {
	obj, err := f.RKE2ControlPlane(cs)
	if err != nil {
		return "", err
	}
	version, _, err := unstructured.NestedString(obj.Object, "spec", "version")
	return version, err
}

// SetRKE2ControlPlaneVersion writes spec.version, which is how a CAPRKE2 cluster's Kubernetes
// version is changed: the CAPRKE2 control-plane controller rolls the control-plane machines to
// converge on it. Retried on conflict, since the control plane is written by several controllers
// concurrently.
func (f *CAPRKE2Fixture) SetRKE2ControlPlaneVersion(cs *clients.Clients, version string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj, err := f.RKE2ControlPlane(cs)
		if err != nil {
			return err
		}
		if err := unstructured.SetNestedField(obj.Object, version, "spec", "version"); err != nil {
			return err
		}
		return cs.Client.Update(cs.Ctx, obj)
	})
}

// nonRestorableMarkerLabel is a machine-template label the restore-mode tests use as a control: it
// is captured in a snapshot (it lives under spec) but is not in restoremode.WritablePaths, so a
// restore must leave it alone. A machineTemplate metadata label is chosen deliberately — CAPI
// propagates those in place rather than rolling machines, so setting it does not disturb the
// cluster.
const nonRestorableMarkerLabel = "restoremode.test.cattle.io/marker"

// NonRestorableMarker reads the control label set by SetNonRestorableMarker.
func (f *CAPRKE2Fixture) NonRestorableMarker(cs *clients.Clients) (string, error) {
	obj, err := f.RKE2ControlPlane(cs)
	if err != nil {
		return "", err
	}
	labels, _, err := unstructured.NestedStringMap(obj.Object, "spec", "machineTemplate", "metadata", "labels")
	if err != nil {
		return "", err
	}
	return labels[nonRestorableMarkerLabel], nil
}

// SetNonRestorableMarker stamps the control label onto the control plane's machine template.
func (f *CAPRKE2Fixture) SetNonRestorableMarker(cs *clients.Clients, value string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj, err := f.RKE2ControlPlane(cs)
		if err != nil {
			return err
		}
		labels, _, err := unstructured.NestedStringMap(obj.Object, "spec", "machineTemplate", "metadata", "labels")
		if err != nil {
			return err
		}
		if labels == nil {
			labels = map[string]string{}
		}
		labels[nonRestorableMarkerLabel] = value
		if err := unstructured.SetNestedStringMap(obj.Object, labels, "spec", "machineTemplate", "metadata", "labels"); err != nil {
			return err
		}
		return cs.Client.Update(cs.Ctx, obj)
	})
}

// Machines returns every CAPI Machine belonging to the cluster, keyed by name with its UID as the
// value. The UID is the identity that matters: a rolled machine can in principle reuse a name, and a
// test asserting "the same machines are still here" means the same objects, not the same names.
func (f *CAPRKE2Fixture) Machines(cs *clients.Clients) (map[string]types.UID, error) {
	list, err := cs.CAPI.Machine().List(f.Namespace, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: f.ClusterName}).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing machines for cluster %s/%s: %w", f.Namespace, f.ClusterName, err)
	}

	machines := map[string]types.UID{}
	for _, machine := range list.Items {
		machines[machine.Name] = machine.UID
	}
	return machines, nil
}

// WaitForMachineReplacement blocks until none of the machines in `previous` are left and the cluster
// has settled on a new set, which is what a control-plane version change is supposed to cause.
// Returns the new set.
func (f *CAPRKE2Fixture) WaitForMachineReplacement(t *testing.T, cs *clients.Clients, previous map[string]types.UID) map[string]types.UID {
	t.Helper()

	var current map[string]types.UID
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 15*time.Second, 45*time.Minute, true, func(context.Context) (bool, error) {
		var err error
		current, err = f.Machines(cs)
		if err != nil {
			return false, nil
		}
		if len(current) == 0 {
			return false, nil
		}
		for name, uid := range previous {
			if current[name] == uid {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for the control plane to replace machines %v (now %v): %v", previous, current, err)
	}

	t.Logf("control-plane machines replaced: %v -> %v", mapNames(previous), mapNames(current))
	return current
}

// healthyCAPIClusterConditions is what "the cluster is completely healthy" means for a CAPI cluster,
// expressed in the v1beta2 conditions the Cluster carries. Beyond the obvious availability ones it
// includes the in-flight conditions: RollingOut/ScalingUp/ScalingDown/Remediating must all be False,
// and ControlPlaneMachinesUpToDate True, or the control plane has decided its machines no longer match
// its spec and is about to replace them.
var healthyCAPIClusterConditions = map[string]string{
	"Available":                    "True",
	"RemoteConnectionProbe":        "True",
	"InfrastructureReady":          "True",
	"ControlPlaneInitialized":      "True",
	"ControlPlaneAvailable":        "True",
	"ControlPlaneMachinesReady":    "True",
	"ControlPlaneMachinesUpToDate": "True",
	"RollingOut":                   "False",
	"Remediating":                  "False",
	"ScalingUp":                    "False",
	"ScalingDown":                  "False",
	"Deleting":                     "False",
	"Paused":                       "False",
}

// WaitForCAPIClusterHealthy blocks until every condition in healthyCAPIClusterConditions holds, and
// fails the test with the offending conditions if it does not. Use it after an operation that is
// expected to leave the cluster intact — it catches both a control plane that never recovered and one
// that recovered by quietly deciding to roll.
func (f *CAPRKE2Fixture) WaitForCAPIClusterHealthy(t *testing.T, cs *clients.Clients, timeout time.Duration) {
	t.Helper()

	t.Logf("waiting for CAPI Cluster %s/%s to be healthy", f.Namespace, f.ClusterName)

	var unmet []string
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 15*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		capiCluster := &unstructured.Unstructured{}
		capiCluster.SetGroupVersionKind(gvkCluster)
		if err := cs.Client.Get(ctx, client.ObjectKey{Namespace: f.Namespace, Name: f.ClusterName}, capiCluster); err != nil {
			unmet = []string{fmt.Sprintf("get: %v", err)}
			return false, nil
		}

		unmet = unmetConditions(capiCluster)
		return len(unmet) == 0, nil
	})
	if err != nil {
		t.Fatalf("CAPI Cluster %s/%s is not healthy: %s", f.Namespace, f.ClusterName, strings.Join(unmet, "; "))
	}
}

// unmetConditions returns a human-readable entry per condition of healthyCAPIClusterConditions that
// is missing or does not have the wanted status.
func unmetConditions(capiCluster *unstructured.Unstructured) []string {
	conds, _, _ := unstructured.NestedSlice(capiCluster.Object, "status", "conditions")

	got := map[string]map[string]any{}
	for _, c := range conds {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		got[condType] = cond
	}

	var unmet []string
	for condType, want := range healthyCAPIClusterConditions {
		cond, found := got[condType]
		if !found {
			unmet = append(unmet, fmt.Sprintf("%s missing", condType))
			continue
		}
		if status, _ := cond["status"].(string); status != want {
			reason, _ := cond["reason"].(string)
			message, _ := cond["message"].(string)
			unmet = append(unmet, fmt.Sprintf("%s=%s (want %s) reason=%s message=%q", condType, status, want, reason, message))
		}
	}
	sort.Strings(unmet)
	return unmet
}

func mapNames(m map[string]types.UID) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
