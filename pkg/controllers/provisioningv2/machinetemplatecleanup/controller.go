package machinetemplatecleanup

import (
	"context"
	"fmt"
	"sync"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/pager"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	cleanupEnabledLabelKey   = "provisioning.cattle.io/infra-cleanup-enabled"
	nextCleanupAnnotationKey = "provisioning.cattle.io/next-infra-cleanup"

	templateGracePeriod = time.Hour
	cleanupPeriod       = 12 * time.Hour

	logPrefix = "capi-infra-cleanup"

	pageSize       = 100
	pageBufferSize = 10
)

type handler struct {
	ctx context.Context

	restMapper meta.RESTMapper
	restConfig *rest.Config

	mgmtClusterController mgmtcontrollers.ClusterController

	capiClusterCache capicontrollers.ClusterCache
	provClusterCache provcontrollers.ClusterCache

	machineDeploymentClient capicontrollers.MachineDeploymentClient
	machineSetClient        capicontrollers.MachineSetClient

	sync.Mutex
}

var infraClusterGKs = []schema.GroupKind{
	{
		Group: capi.GroupVersionInfrastructure.Group,
		Kind:  "AWSCluster",
	},
}

var infraMachineTemplateGKs = []schema.GroupKind{
	{
		Group: capi.GroupVersionInfrastructure.Group,
		Kind:  "AWSMachineTemplate",
	},
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := handler{
		ctx: ctx,

		restMapper: clients.RESTMapper,
		restConfig: clients.RESTConfig,

		mgmtClusterController: clients.Mgmt.Cluster(),

		capiClusterCache: clients.CAPI.Cluster().Cache(),
		provClusterCache: clients.Provisioning.Cluster().Cache(),

		machineDeploymentClient: clients.CAPI.MachineDeployment(),
		machineSetClient:        clients.CAPI.MachineSet(),
	}

	h.mgmtClusterController.OnChange(ctx, "capi-infra-cleanup", h.OnClusterChange)
}

/*
This controller starts a periodic routine to clean-up unused CAPI
infrastructure machine templates and CAPI infrastructure clusters.

The clean-up happens in a separate thread and only periodically
because it can potentially list and handle many objects,

It's meant to clean-up infrastructure clusters and infrastructure
machine templates used by Rancher clusters.provisioning.cattle.io
cluster objects and created by the UI extensions that support this,
e.g. for CAPR+CAPA.

It's not meant to be used as a general method of cleaning-up
infrastructure machine templates and infrastructure clusters, will
only consider for deletion objects with a specific label.

There are two situations that this controller covers:

- Infrastructure machine templates/clusters that are created but never
adopted (e.g. the UI creates a template but fails to create the
corresponding clusters.provisioning.cattle.io cluster). These objects
are given a grace period to be adopted and are deleted if they never
get any owners within that period.
- Infrastructure machine templates that were adopted but are no longer
used. This is expected since templates are usually immutable and
changing the configuration of a MachineDeployment requires creating a
new template and pointing the MD to it.

We assume that once a template stops being used, its name will no longer
be used for new templates.
*/
func (h *handler) OnClusterChange(_ string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Name != "local" {
		return cluster, nil
	}

	var err error
	var nextCleanupTime time.Time
	if annotationVal, ok := cluster.Annotations[nextCleanupAnnotationKey]; ok {
		nextCleanupTime, err = time.Parse(time.RFC3339, annotationVal)
		if err != nil {
			logrus.Warnf("[%s] local cluster had an invalid value for annotation %s, updating...",
				logPrefix,
				nextCleanupAnnotationKey,
			)
			return h.updateNextCleanupTime(cluster)
		}
	} else {
		return h.updateNextCleanupTime(cluster)
	}

	if nextCleanupTime.After(time.Now()) {
		h.mgmtClusterController.EnqueueAfter(cluster.Name, time.Until(nextCleanupTime))
		return cluster, nil
	}

	// If we get an error after this, we'll only try again on the next
	// execution.
	cluster, err = h.updateNextCleanupTime(cluster)
	if err != nil {
		return nil, err
	}

	logrus.Infof("[%s] about to clean up unused capi infra templates and clusters", logPrefix)

	// Only run one clean-up routine at a time.
	if !h.TryLock() {
		logrus.Infof("[%s] capi infra templates are already being cleaned-up, skipping", logPrefix)
		return cluster, nil
	}

	go func() {
		defer h.Unlock()

		err = h.cleanup()
		if err != nil {
			logrus.Errorf("[%s] error cleaning up: %v", logPrefix, err)
		}

		logrus.Infof("[%s] done cleaning up", logPrefix)
	}()

	return cluster, nil
}

func (h *handler) cleanup() error {
	dynamicClient, err := dynamic.NewForConfig(h.restConfig)
	if err != nil {
		return err
	}

	for _, infraClusterGK := range infraClusterGKs {
		logrus.Infof("[%s] cleaning up %s resources...", logPrefix, infraClusterGK)

		err = h.cleanupInfraClusters(h.ctx, infraClusterGK, dynamicClient)
		if err != nil {
			logrus.Errorf("[%s] error cleaning up %s resources: %v", logPrefix, infraClusterGK, err)
		}
	}

	for _, infraMachineTemplateGK := range infraMachineTemplateGKs {
		logrus.Infof("[%s] cleaning up %s resources...", logPrefix, infraMachineTemplateGK)

		err = h.cleanupInfraMachineTemplates(h.ctx, infraMachineTemplateGK, dynamicClient)
		if err != nil {
			logrus.Errorf("[%s] error cleaning up %s resources: %v", logPrefix, infraMachineTemplateGK, err)
		}
	}

	return nil
}

func (h *handler) updateNextCleanupTime(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	cluster = cluster.DeepCopy()

	annotations := cluster.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[nextCleanupAnnotationKey] = time.Now().Add(cleanupPeriod).Format(time.RFC3339)

	cluster.Annotations = annotations

	return h.mgmtClusterController.Update(cluster)
}

func (h *handler) cleanupInfraClusters(
	ctx context.Context,
	infraClusterGK schema.GroupKind,
	dynamicClient *dynamic.DynamicClient) error {

	infraClusterMapping, err := h.restMapper.RESTMapping(infraClusterGK)
	if err != nil {
		return fmt.Errorf("getting REST mapping for %s: %w", infraClusterGK, err)
	}

	infraClusterClient := dynamicClient.Resource(infraClusterMapping.Resource)

	p := pager.ListPager{
		PageSize:          pageSize,
		PageBufferSize:    pageBufferSize,
		FullListIfExpired: false,
		PageFn: func(ctx context.Context, opts v1.ListOptions) (runtime.Object, error) {
			return infraClusterClient.List(ctx, opts)
		},
	}

	var hasErr bool
	err = p.EachListItem(
		ctx,
		v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=true", cleanupEnabledLabelKey),
		},
		func(obj runtime.Object) error {
			if err := h.cleanupInfraCluster(ctx, obj, infraClusterClient); err != nil {
				logrus.Errorf("[%s] error cleaning up %s: %v", logPrefix, infraClusterGK, err)
				hasErr = true
			}
			return nil
		},
	)
	if err != nil {
		return err
	}

	if hasErr {
		return fmt.Errorf("some resources had errors")
	}

	return nil
}

// Infrastructure clusters should usually not change throughout the lifetime
// of a cluster, so we only handle clusters that were never adopted.
func (h *handler) cleanupInfraCluster(
	ctx context.Context,
	infraCluster runtime.Object,
	infraClusterClient dynamic.NamespaceableResourceInterface) error {

	infraClusterMeta, err := meta.Accessor(infraCluster)
	if err != nil {
		return err
	}

	logrus.Infof("[%s] checking %s/%s...",
		logPrefix,
		infraClusterMeta.GetNamespace(),
		infraClusterMeta.GetName(),
	)

	if infraClusterMeta.GetCreationTimestamp().Add(templateGracePeriod).After(time.Now()) {
		return nil
	}

	if len(infraClusterMeta.GetOwnerReferences()) > 0 {
		return nil
	}

	namespacedClient := infraClusterClient.Namespace(infraClusterMeta.GetNamespace())

	logrus.Infof("[%s] deleting %s/%s...", logPrefix, infraClusterMeta.GetNamespace(), infraClusterMeta.GetName())

	uid := infraClusterMeta.GetUID()
	rv := infraClusterMeta.GetResourceVersion()

	err = namespacedClient.Delete(ctx, infraClusterMeta.GetName(), v1.DeleteOptions{
		Preconditions: &v1.Preconditions{
			// Only delete the object if it's the same we
			// listed before.
			UID: &uid,

			// This prevents deleting when even after the grace period
			// the object is adopted by the capi cluster (which changes the
			// owner references and the rv for this object). It doesn't
			// generally prevent deleting an object being used as it can still
			// be referenced by the provisioning cluster and not yet adopted
			// by the capi cluster, so the grace period is still needed.
			ResourceVersion: &rv,
		},
	})

	if err != nil {
		return fmt.Errorf("deleting %s/%s: %w",
			infraClusterMeta.GetNamespace(),
			infraClusterMeta.GetName(),
			err,
		)
	}

	return nil
}

func (h *handler) cleanupInfraMachineTemplates(
	ctx context.Context,
	infraMachineTemplateGK schema.GroupKind,
	dynamicClient *dynamic.DynamicClient) error {

	infraMachineTemplateMapping, err := h.restMapper.RESTMapping(infraMachineTemplateGK)
	if err != nil {
		return fmt.Errorf("getting REST mapping for %s: %w", infraMachineTemplateGK, err)
	}

	infraMachineTemplateClient := dynamicClient.Resource(infraMachineTemplateMapping.Resource)

	p := pager.ListPager{
		PageSize:          pageSize,
		PageBufferSize:    pageBufferSize,
		FullListIfExpired: false,
		PageFn: func(ctx context.Context, opts v1.ListOptions) (runtime.Object, error) {
			return infraMachineTemplateClient.List(ctx, opts)
		},
	}

	var hasErr bool

	// Since we could get multiple templates owned by a single cluster,
	// use a map to cache the list of templates being used by cluster.
	infraMachineTemplatesByOwnerCluster := make(map[k8sTypes.UID]map[string]bool)
	err = p.EachListItem(
		ctx,
		v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=true", cleanupEnabledLabelKey),
		},
		func(obj runtime.Object) error {
			if err := h.cleanupInfraMachineTemplate(
				ctx,
				obj,
				infraMachineTemplateGK,
				infraMachineTemplatesByOwnerCluster,
				infraMachineTemplateClient,
			); err != nil {
				logrus.Errorf("[%s] error cleaning up %s: %v", logPrefix, infraMachineTemplateGK, err)
				hasErr = true
			}
			return nil
		},
	)
	if err != nil {
		return err
	}

	if hasErr {
		return fmt.Errorf("some resources had errors")
	}

	return nil
}

func (h *handler) cleanupInfraMachineTemplate(
	ctx context.Context,
	infraMachineTemplate runtime.Object,
	infraMachineTemplateGK schema.GroupKind,
	infraMachineTemplatesByOwnerCluster map[k8sTypes.UID]map[string]bool,
	infraMachineTemplateClient dynamic.NamespaceableResourceInterface,
) error {
	infraMachineTemplateMeta, err := meta.Accessor(infraMachineTemplate)
	if err != nil {
		return err
	}

	logrus.Infof("[%s] checking %s/%s...",
		logPrefix,
		infraMachineTemplateMeta.GetNamespace(),
		infraMachineTemplateMeta.GetName(),
	)

	if infraMachineTemplateMeta.GetCreationTimestamp().Add(templateGracePeriod).After(time.Now()) {
		return nil
	}

	infraMachineOwnerTemplateRefs := infraMachineTemplateMeta.GetOwnerReferences()

	delete := false
	switch len(infraMachineOwnerTemplateRefs) {
	case 0:
		// Template was never adopted.
		delete = true
	case 1:
		// Template was adopted but might no longer be in use.
		infraMachineTemplateOwnerRef := infraMachineOwnerTemplateRefs[0]
		infraMachineTemplatesInUse, err := h.findUsedTemplatesByOwner(
			infraMachineTemplateMeta,
			infraMachineTemplateOwnerRef,
			infraMachineTemplateGK,
			infraMachineTemplatesByOwnerCluster,
		)
		if err != nil {
			return fmt.Errorf("determining templates used by owner of %s/%s: %w",
				infraMachineTemplateMeta.GetNamespace(),
				infraMachineTemplateMeta.GetName(),
				err,
			)
		}

		delete = !infraMachineTemplatesInUse[infraMachineTemplateMeta.GetName()]
	default:
		return fmt.Errorf("%s/%s has too many owners",
			infraMachineTemplateMeta.GetNamespace(),
			infraMachineTemplateMeta.GetName(),
		)
	}

	if delete {
		logrus.Infof("[%s] deleting %s/%s...",
			logPrefix,
			infraMachineTemplateMeta.GetNamespace(),
			infraMachineTemplateMeta.GetName(),
		)

		namespacedClient := infraMachineTemplateClient.Namespace(infraMachineTemplateMeta.GetNamespace())

		uid := infraMachineTemplateMeta.GetUID()
		rv := infraMachineTemplateMeta.GetResourceVersion()

		err = namespacedClient.Delete(ctx, infraMachineTemplateMeta.GetName(), v1.DeleteOptions{
			Preconditions: &v1.Preconditions{
				UID:             &uid,
				ResourceVersion: &rv,
			},
		})
		if err != nil {
			return fmt.Errorf("deleting %s/%s: %w",
				infraMachineTemplateMeta.GetNamespace(),
				infraMachineTemplateMeta.GetName(),
				err,
			)
		}
	}

	return nil
}

// Find which templates are used by a given cluster that owns a
// template. We check both the provisioning cluster definition and
// its MachineDeployments/MachineSets. It could happen e.g. that
// the provisioning cluster was updated but not its MD, which could
// still need the template for machine rollouts.
//
// It should usually not be a problem to delete a template that is no
// longer used by the MD but is used by the MS, but we might as well check
// them too.
func (h *handler) findUsedTemplatesByOwner(
	infraMachineTemplateMeta v1.Object,
	infraMachineTemplateOwnerRef v1.OwnerReference,
	infraMachineTemplateGK schema.GroupKind,
	infraMachineTemplatesByOwnerCluster map[k8sTypes.UID]map[string]bool,
) (map[string]bool, error) {
	infraMachineTemplateOwnerGV, err := schema.ParseGroupVersion(infraMachineTemplateOwnerRef.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing GroupVersion of owner: %w", err)
	}

	if infraMachineTemplateOwnerGV.Group != capi.GroupVersion.Group ||
		infraMachineTemplateOwnerRef.Kind != "Cluster" ||
		infraMachineTemplateOwnerRef.UID == "" {
		return nil, fmt.Errorf("unexpected owner")
	}

	if infraMachineTemplatesInUse, ok := infraMachineTemplatesByOwnerCluster[infraMachineTemplateOwnerRef.UID]; ok {
		return infraMachineTemplatesInUse, nil
	}

	capiCluster, err := h.capiClusterCache.Get(
		infraMachineTemplateMeta.GetNamespace(),
		infraMachineTemplateOwnerRef.Name,
	)
	if err != nil {
		return nil, fmt.Errorf("getting CAPI cluster owner: %w", err)
	}

	if capiCluster.UID != infraMachineTemplateOwnerRef.UID {
		return nil, fmt.Errorf("unexpected CAPI cluster in cache")
	}

	var capiClusterOwnerRef *v1.OwnerReference
	for _, ownerRef := range capiCluster.OwnerReferences {
		ownerGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("parsing GroupVersion of CAPI cluster owner: %w", err)
		}

		if ownerGV.Group == provv1.SchemeGroupVersion.Group && ownerRef.Kind == "Cluster" {
			capiClusterOwnerRef = &ownerRef
		}
	}

	if capiClusterOwnerRef == nil {
		return nil, fmt.Errorf("CAPI cluster owner has no provisioning cluster owner")
	}

	provCluster, err := h.provClusterCache.Get(capiCluster.Namespace, capiClusterOwnerRef.Name)
	if err != nil {
		return nil, fmt.Errorf("getting provisioning cluster owner for CAPI cluster: %w", err)
	}

	if provCluster.UID != capiClusterOwnerRef.UID {
		return nil, fmt.Errorf("unexpected CAPI cluster in cache")
	}

	var machinePools []provv1.RKEMachinePool
	if provCluster.Spec.RKEConfig != nil {
		machinePools = provCluster.Spec.RKEConfig.MachinePools
	}

	infraMachineTemplatesInUse := make(map[string]bool)

	for _, mp := range machinePools {
		if mp.NodeConfig != nil {
			mpGV, err := schema.ParseGroupVersion(mp.NodeConfig.APIVersion)
			if err != nil {
				return nil, fmt.Errorf("parsing machine pool node config GroupVersion: %w", err)
			}

			if mpGV.Group == infraMachineTemplateGK.Group &&
				mp.NodeConfig.Kind == infraMachineTemplateGK.Kind {
				infraMachineTemplatesInUse[mp.NodeConfig.Name] = true
			}
		}
	}

	machineDeployments, err := h.machineDeploymentClient.List(provCluster.Namespace, v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, provCluster.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("listing machine deployments: %w", err)
	}

	for _, md := range machineDeployments.Items {
		if md.Spec.Template.Spec.InfrastructureRef.APIGroup == infraMachineTemplateGK.Group &&
			md.Spec.Template.Spec.InfrastructureRef.Kind == infraMachineTemplateGK.Kind {
			infraMachineTemplatesInUse[md.Spec.Template.Spec.InfrastructureRef.Name] = true
		}
	}

	machineSets, err := h.machineSetClient.List(provCluster.Namespace, v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, provCluster.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("listing machine sets: %w", err)
	}

	for _, ms := range machineSets.Items {
		if ms.Spec.Template.Spec.InfrastructureRef.APIGroup == infraMachineTemplateGK.Group &&
			ms.Spec.Template.Spec.InfrastructureRef.Kind == infraMachineTemplateGK.Kind {
			infraMachineTemplatesInUse[ms.Spec.Template.Spec.InfrastructureRef.Name] = true
		}
	}

	infraMachineTemplatesByOwnerCluster[infraMachineTemplateOwnerRef.UID] = infraMachineTemplatesInUse
	return infraMachineTemplatesInUse, nil
}
