package snapshotextrametadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	"github.com/rancher/rancher/pkg/capr"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	configMapName = "rke2-etcd-snapshot-extra-metadata"

	resourcesKey    = "resources"
	restoreModesKey = "restoreModes"

	// Resource keys addressing the objects published in the resources section. Restore mode
	// selectors are rooted at this section, so a selector's first segment is one of these keys.
	mgmtClusterKey      = "cluster.management.cattle.io"
	provClusterKey      = "cluster.provisioning.cattle.io"
	rke2ControlPlaneKey = "rke2controlplane.controlplane.cluster.x-k8s.io"

	// administratedAnnotation marks a mgmt v3 Cluster shell whose real configuration lives on a
	// provisioning cluster. Matches the check in pkg/operations/imported.go.
	administratedAnnotation = "provisioning.cattle.io/administrated"
)

// errUnsupportedClusterType is returned when a cluster is managed by something this controller has
// no resources to publish for, e.g. a CAPI cluster whose control plane is not CAPRKE2. It is not a
// transient failure, so onChange logs and moves on rather than requeueing forever.
var errUnsupportedClusterType = errors.New("unsupported cluster type")

// renderedFields are the only top-level fields sanitize keeps. Everything else, i.e. status and any
// other subresource-backed field, is purged: subresources are not restorable from a snapshot.
var renderedFields = []string{"apiVersion", "kind", "metadata", "spec"}

// stripPaths marks, per resource key, the fields that are removed from an object before it is
// published. Restoring one of these would immediately re-run the operation that produced the
// snapshot in the first place, so they are dropped for the same reason
// provisioningcluster.rkeControlPlane drops them before stamping capr.ClusterSpecAnnotation (see
// the note above provv1.RKEConfig). Resource keys with nothing to strip are simply absent.
var stripPaths = map[string][][]string{
	provClusterKey: {
		{"spec", "rkeConfig", "etcdSnapshotRestore"},
		{"spec", "rkeConfig", "etcdSnapshotCreate"},
		{"spec", "rkeConfig", "rotateEncryptionKeys"},
		{"spec", "rkeConfig", "rotateCertificates"},
	},
}

// dynamicClient is the slice of lasso's dynamic controller this package needs, kept narrow so tests
// can fake it. Mirrors snapshotbackpopulate.dynamicClient.
type dynamicClient interface {
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
}

type handler struct {
	clusterName string

	configMap        corecontrollers.ConfigMapClient
	dynamic          dynamicClient
	provClusterCache rocontrollers.ClusterCache
	capiClusterCache capicontrollers.ClusterCache
}

func Register(ctx context.Context, userContext *config.UserContext, capiCtx *wrangler.CAPIContext, cluster *apimgmtv3.Cluster) {
	logrus.Debugf("[snapshotextrametadata] Registering controller for cluster %s", userContext.ClusterName)

	h := &handler{
		clusterName:      userContext.ClusterName,
		configMap:        userContext.Corew.ConfigMap(),
		dynamic:          userContext.Management.Wrangler.Dynamic,
		provClusterCache: userContext.Management.Wrangler.Provisioning.Cluster().Cache(),
		capiClusterCache: capiCtx.CAPI.Cluster().Cache(),
	}

	userContext.Management.Wrangler.Mgmt.Cluster().OnChange(ctx, "snapshotextrametadata", h.onChange)
}

func (h *handler) onChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	// The handler is registered per downstream cluster but watches every mgmt Cluster, so ignore
	// the ones this agent is not responsible for.
	if cluster == nil || cluster.Name != h.clusterName {
		return cluster, nil
	}

	a, err := h.newAdapter(cluster)
	if errors.Is(err, errUnsupportedClusterType) {
		logrus.Debugf("[snapshotextrametadata] cluster %s: %v, not publishing extra metadata", cluster.Name, err)
		return cluster, nil
	}
	if err != nil {
		return nil, err
	}

	data, err := renderData(a)
	if err != nil {
		return nil, err
	}

	cm, err := h.configMap.Get(metav1.NamespaceSystem, configMapName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = h.configMap.Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: metav1.NamespaceSystem,
				Name:      configMapName,
			},
			Data: data,
		})
		if err != nil {
			return nil, err
		}
		return cluster, nil
	}
	if err != nil {
		return nil, err
	}

	if maps.Equal(cm.Data, data) {
		return cluster, nil
	}

	cm = cm.DeepCopy()
	cm.Data = data

	if _, err = h.configMap.Update(cm); err != nil {
		return nil, err
	}

	return cluster, nil
}

// resource is a single object published in the resources section, keyed by the resource type restore
// mode selectors address it by.
type resource struct {
	key string
	obj any
}

// adapter describes how a downstream cluster is managed upstream. Each implementation knows which
// objects have to survive a snapshot restore for its cluster type and where the Kubernetes version
// lives within them, in the same spirit as the cluster adapters in pkg/operations/adapter.go.
type adapter interface {
	// resources returns every object to publish for this cluster.
	resources() ([]resource, error)

	// kubernetesVersionSelector returns the selector, rooted at the resources section, for the
	// field the kubernetesVersion restore mode restores.
	kubernetesVersionSelector() string
}

// newAdapter resolves how cluster is managed and returns the matching adapter. The dispatch order
// mirrors the mgmt-cluster factory in pkg/operations/imported.go: turtles-imported CAPI first, then
// v2prov-administrated, then a true imported cluster.
func (h *handler) newAdapter(cluster *apimgmtv3.Cluster) (adapter, error) {
	if a, err := h.turtlesAdapter(cluster); a != nil || err != nil {
		return a, err
	}
	if a, err := h.administratedAdapter(cluster); a != nil || err != nil {
		return a, err
	}
	return &importedAdapter{cluster: cluster}, nil
}

// turtlesAdapter returns an adapter for a turtles-imported CAPI cluster shell, identified by the
// presence of both capi-cluster-owner labels. Returns (nil, nil) when the labels are absent so the
// caller can try the next dispatch; one label without the other is a misconfiguration and errors
// rather than silently falling through.
func (h *handler) turtlesAdapter(cluster *apimgmtv3.Cluster) (adapter, error) {
	ownerName := cluster.Labels[capr.CAPIClusterOwnerLabel]
	ownerNS := cluster.Labels[capr.CAPIClusterOwnerNSLabel]
	if (ownerName == "") != (ownerNS == "") {
		return nil, fmt.Errorf("mgmt cluster %s carries only one of %s/%s; both must be set for a turtles-imported CAPI cluster",
			cluster.Name, capr.CAPIClusterOwnerLabel, capr.CAPIClusterOwnerNSLabel)
	}
	if ownerName == "" {
		return nil, nil
	}

	capiCluster, err := h.capiClusterCache.Get(ownerNS, ownerName)
	if err != nil {
		return nil, fmt.Errorf("mgmt cluster %s references CAPI cluster %s/%s: %w", cluster.Name, ownerNS, ownerName, err)
	}

	if capiCluster.Spec.ControlPlaneRef.APIGroup != controlplanev1beta2.GroupVersion.Group ||
		capiCluster.Spec.ControlPlaneRef.Kind != "RKE2ControlPlane" {
		return nil, fmt.Errorf("%w: CAPI cluster %s/%s control plane is %s %s, not CAPRKE2", errUnsupportedClusterType,
			capiCluster.Namespace, capiCluster.Name, capiCluster.Spec.ControlPlaneRef.APIGroup, capiCluster.Spec.ControlPlaneRef.Kind)
	}

	// The RKE2ControlPlane is fetched dynamically because CAPRKE2's CRDs are only installed once
	// turtles is enabled, so there is no generated typed cache for it. Convert through the
	// unstructured form as pkg/operations/capi.go does.
	obj, err := h.dynamic.Get(controlplanev1beta2.GroupVersion.WithKind("RKE2ControlPlane"), capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return nil, err
	}
	cpUstr, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("expected *unstructured.Unstructured for RKE2ControlPlane %s/%s, got %T",
			capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name, obj)
	}
	controlPlane := &controlplanev1beta2.RKE2ControlPlane{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(cpUstr.Object, controlPlane); err != nil {
		return nil, fmt.Errorf("converting RKE2ControlPlane %s/%s from unstructured: %w",
			capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name, err)
	}

	return &caprke2Adapter{controlPlane: controlPlane}, nil
}

// administratedAdapter returns an adapter for a v2prov cluster (custom or node-driver), identified
// by the administrated annotation on the mgmt cluster shell, by tracing mgmt Cluster -> provv1
// Cluster through the ByCluster index. Returns (nil, nil) when the annotation is not set.
func (h *handler) administratedAdapter(cluster *apimgmtv3.Cluster) (adapter, error) {
	if cluster.Annotations[administratedAnnotation] != "true" {
		return nil, nil
	}

	provClusters, err := h.provClusterCache.GetByIndex(provcluster.ByCluster, cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("finding provisioning cluster for mgmt cluster %s: %w", cluster.Name, err)
	}
	if len(provClusters) != 1 {
		return nil, fmt.Errorf("expected exactly 1 provisioning cluster for mgmt cluster %s, got %d", cluster.Name, len(provClusters))
	}

	return &provisioningAdapter{cluster: provClusters[0]}, nil
}

// importedAdapter handles a true imported cluster. Nothing upstream provisions it, so the mgmt v3
// Cluster is the only record of the configuration Rancher knows about.
type importedAdapter struct {
	cluster *apimgmtv3.Cluster
}

func (a *importedAdapter) resources() ([]resource, error) {
	return []resource{{key: mgmtClusterKey, obj: a.cluster}}, nil
}

func (a *importedAdapter) kubernetesVersionSelector() string {
	return selector(mgmtClusterKey, "spec", "rke2Config", "kubernetesVersion")
}

// provisioningAdapter handles a v2prov cluster. The provisioning Cluster holds the authoritative
// spec, and its operation-triggering fields are stripped on the way out (see stripPaths).
type provisioningAdapter struct {
	cluster *provv1.Cluster
}

func (a *provisioningAdapter) resources() ([]resource, error) {
	return []resource{{key: provClusterKey, obj: a.cluster}}, nil
}

func (a *provisioningAdapter) kubernetesVersionSelector() string {
	return selector(provClusterKey, "spec", "kubernetesVersion")
}

// caprke2Adapter handles a turtles-imported CAPI cluster backed by CAPRKE2. The RKE2ControlPlane
// carries the cluster configuration; neither the CAPI Cluster nor the mgmt v3 shell does.
type caprke2Adapter struct {
	controlPlane *controlplanev1beta2.RKE2ControlPlane
}

func (a *caprke2Adapter) resources() ([]resource, error) {
	return []resource{{key: rke2ControlPlaneKey, obj: a.controlPlane}}, nil
}

func (a *caprke2Adapter) kubernetesVersionSelector() string {
	return selector(rke2ControlPlaneKey, "spec", "version")
}

// selector renders a JSONPath into the resources section. The resource key is bracket-quoted
// because it contains dots.
func selector(key string, fields ...string) string {
	return fmt.Sprintf("$['%s'].%s", key, strings.Join(fields, "."))
}

// renderData builds the data of the etcd snapshot extra metadata ConfigMap. RKE2/K3s copy every key
// in it into the snapshot's metadata, so the resources section carries the objects to restore from
// and the restoreModes section carries the selector each restore mode applies to those resources.
func renderData(a adapter) (map[string]string, error) {
	resourceList, err := a.resources()
	if err != nil {
		return nil, err
	}

	resources := map[string]any{}
	for _, r := range resourceList {
		sanitized, err := sanitize(r.key, r.obj)
		if err != nil {
			return nil, err
		}
		resources[r.key] = sanitized
	}

	// Compressed the same way as capr.ClusterSpecAnnotation, so consumers decode it with
	// snapshotutil.DecompressInterface.
	compressed, err := snapshotutil.CompressInterface(resources)
	if err != nil {
		return nil, err
	}

	restoreModes := map[string]any{
		"all":               "*",
		"kubernetesVersion": a.kubernetesVersionSelector(),
		"none":              "",
	}

	out, err := json.Marshal(restoreModes)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		resourcesKey:    compressed,
		restoreModesKey: string(out),
	}, nil
}

// sanitize converts obj to its unstructured form, purged of all subresources and of every field
// registered in stripPaths for key.
func sanitize(key string, obj any) (map[string]any, error) {
	umap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	for field := range umap {
		if !slices.Contains(renderedFields, field) {
			delete(umap, field)
		}
	}

	for _, path := range stripPaths[key] {
		unstructured.RemoveNestedField(umap, path...)
	}

	return umap, nil
}
