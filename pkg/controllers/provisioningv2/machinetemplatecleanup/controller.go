package machinetemplatecleanup

import (
	"context"
	"fmt"
	"sync"
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/fleet"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	CleanupEnabledLabelKey   = "provisioning.cattle.io/infra-cleanup-enabled"
	nextCleanupAnnotationKey = "provisioning.cattle.io/next-infra-cleanup"

	templateGracePeriod = time.Hour
	cleanupPeriod       = 12 * time.Hour
	retryPeriod         = 15 * time.Minute
	pageSize            = 100

	logPrefix = "[capi-infra-cleanup]"
)

/*
This controller handles cleaning up unused CAPI infrastructure

machine templates and CAPI infrastructure clusters.

The clean-up happens in a separate thread and only periodically
because it can potentially list and handle many objects.

It's meant to clean-up infrastructure clusters and infrastructure
machine templates used by Rancher clusters.provisioning.cattle.io
cluster objects and created by the UI extensions that support this,
e.g. for CAPR+CAPA.

It's not meant to be used as a general method of cleaning-up
infrastructure machine templates and infrastructure clusters, and will
only consider for deletion objects with a specific label.

There are two situations that this controller covers:

- Infrastructure machine templates/clusters that are created but never
adopted (e.g. the UI creates a template but fails to create the
corresponding clusters.provisioning.cattle.io cluster).
- Infrastructure machine templates that were adopted but are no longer
used. This is expected since templates are usually immutable and
changing the configuration of a MachineDeployment requires creating a
new template and pointing the MD to it.

The cleanup routine will only execute periodically since it can do
many apiserver calls, this is done by annotating provisioning clusters
with with the timestamp of the next execution.

The local provisioning cluster is used to track and clean up unowned
objects. If it finds an owned template, it will also label it with the
owner's name, so that the owned object routine can more easily find
templates associated with a given cluster.

Each other provisioning cluster that potentially references templates
is used to track clean ups of that cluster's templates. This allows
only computing the set of used templates once per cluster. If we
considered each template individually, we would end up computing this
set multiple, given that multiple templates can be owned by a single
cluster.
*/
type handler struct {
	ctx context.Context

	provClusterController provcontrollers.ClusterController

	capiClusterCache capicontrollers.ClusterCache

	dynamicClient           dynamic.Interface
	machineDeploymentClient capicontrollers.MachineDeploymentClient
	machineSetClient        capicontrollers.MachineSetClient

	cleanupQueue chan (*provv1.Cluster)

	gkToGVR   map[schema.GroupKind]schema.GroupVersionResource
	gkToGVRmu sync.Mutex
}

var InfraClusterToMachineTemplateGKs = map[schema.GroupKind]schema.GroupKind{
	{
		Group: capi.GroupVersionInfrastructure.Group,
		Kind:  "AWSCluster",
	}: {
		Group: capi.GroupVersionInfrastructure.Group,
		Kind:  "AWSMachineTemplate",
	},
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) error {
	h := handler{
		ctx: ctx,

		provClusterController: clients.Provisioning.Cluster(),

		capiClusterCache: clients.CAPI.Cluster().Cache(),

		machineDeploymentClient: clients.CAPI.MachineDeployment(),
		machineSetClient:        clients.CAPI.MachineSet(),

		cleanupQueue: make(chan *provv1.Cluster, 5),
	}

	var err error
	h.dynamicClient, err = dynamic.NewForConfig(clients.RESTConfig)
	if err != nil {
		return err
	}

	h.provClusterController.OnChange(ctx, "capi-infra-cleanup", h.OnClusterChange)

	clients.CRD.CustomResourceDefinition().OnChange(ctx, "capi-infra-cleanup-discovery", h.OnCRD)

	h.initGVRs()

	go h.cleanup()

	return nil
}

func (h *handler) initGVRs() {
	h.gkToGVRmu.Lock()
	defer h.gkToGVRmu.Unlock()
	h.gkToGVR = make(map[schema.GroupKind]schema.GroupVersionResource)
	for infraClusterGK, infraMachineTemplateGK := range InfraClusterToMachineTemplateGKs {
		h.gkToGVR[infraClusterGK] = schema.GroupVersionResource{}
		h.gkToGVR[infraMachineTemplateGK] = schema.GroupVersionResource{}
	}
}

// The presence and version of CRDs (and API endpoints) for the objects we handle depends
// on which CAPI providers are installed, so we need to dynamically find out the GVRs for
// these objects.
func (h *handler) OnCRD(key string, crd *apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
	if crd == nil {
		return crd, nil
	}

	crdGK := schema.GroupKind{
		Group: crd.Spec.Group,
		Kind:  crd.Status.AcceptedNames.Kind,
	}

	h.gkToGVRmu.Lock()
	defer h.gkToGVRmu.Unlock()

	if _, ok := h.gkToGVR[crdGK]; !ok {
		return crd, nil
	}

	// Get any served version, we'll only use metadata
	// for the clean up.
	version := ""
	for _, ver := range crd.Spec.Versions {
		if ver.Served {
			version = ver.Name
		}
	}

	if version == "" {
		logrus.Warnf("%s CRD %s has no served versions", logPrefix, crd.Name)
		return crd, nil
	}

	if crd.Status.AcceptedNames.Plural == "" {
		logrus.Warnf("%s CRD %s has no plural name", logPrefix, crd.Name)
		return crd, nil
	}

	// Note that if a provider is removed and its CRDs deleted, we'll have a stale GVR until Rancher
	// is restarted and get listing errors. We accept this since we only run this routine periodically.
	h.gkToGVR[crdGK] = schema.GroupVersionResource{
		Group:    crdGK.Group,
		Version:  version,
		Resource: crd.Status.AcceptedNames.Plural,
	}

	return crd, nil
}

func (h *handler) clientForGK(gk schema.GroupKind) (dynamic.NamespaceableResourceInterface, error) {
	h.gkToGVRmu.Lock()
	defer h.gkToGVRmu.Unlock()

	gvr := h.gkToGVR[gk]

	if gvr.Empty() {
		return nil, fmt.Errorf("client for %s not available", gk)
	}

	return h.dynamicClient.Resource(gvr), nil
}

func (h *handler) OnClusterChange(key string, cluster *provv1.Cluster) (*provv1.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	if !(h.isLocal(cluster) || h.shouldCleanup(cluster)) {
		return cluster, nil
	}

	now := time.Now()

	var err error
	var nextCleanupTime time.Time

	if annotationVal, ok := cluster.Annotations[nextCleanupAnnotationKey]; ok {
		nextCleanupTime, err = time.Parse(time.RFC3339, annotationVal)
		if err != nil {
			logrus.Warnf("%s cluster %s/%s had an invalid value for annotation: %s, updating...",
				logPrefix,
				cluster.Namespace,
				cluster.Name,
				nextCleanupAnnotationKey,
			)
			return h.updateNextCleanupTime(cluster, cleanupPeriod, now)
		}
	} else {
		return h.updateNextCleanupTime(cluster, cleanupPeriod, now)
	}

	if nextCleanupTime.After(now) {
		h.provClusterController.EnqueueAfter(cluster.Namespace, cluster.Name, nextCleanupTime.Sub(now))
		return cluster, nil
	}

	logrus.Debugf("%s about to try to schedule cleanup for %s", logPrefix, key)

	// Update before trying to issue the clean-up to avoid doing it more than
	// once for the same period: the controller can see the same RV for this cluster multiple times,
	// and if that happens, it will fail here due to optimistic locking.
	cluster, err = h.updateNextCleanupTime(cluster, cleanupPeriod, now)
	if err != nil {
		logrus.Debugf("%s error updating cleanup timestamp for %s: %v", logPrefix, key, err)
		return nil, err
	}

	select {
	case h.cleanupQueue <- cluster.DeepCopy():
		logrus.Debugf("%s scheduled clean up for %s", logPrefix, key)
	default:
		// Try again in a shorter period. If this fails, we'll have to
		// wait another full period.
		logrus.Debugf("%s cleanup queue was busy for %s, retrying in %s", logPrefix, key, retryPeriod)
		return h.updateNextCleanupTime(cluster, retryPeriod, now)
	}

	return cluster, nil
}

// We use the local cluster as the object to track clean up of resources that were never
// attached to any provisioned clusters. Other clusters are used to handle their previously
// owned resources.
func (h *handler) isLocal(cluster *provv1.Cluster) bool {
	return cluster.Namespace == fleet.ClustersLocalNamespace && cluster.Name == "local"
}

func (h *handler) shouldCleanup(cluster *provv1.Cluster) bool {
	if cluster.Spec.RKEConfig == nil || cluster.Spec.RKEConfig.InfrastructureRef == nil {
		return false
	}

	infraClusterGV, err := schema.ParseGroupVersion(cluster.Spec.RKEConfig.InfrastructureRef.APIVersion)
	if err != nil {
		logrus.Warnf("%s could not parse GV for infra cluster ref of %s/%s: %v", logPrefix, cluster.Namespace, cluster.Name, err)
		return false
	}

	infraClusterGK := infraClusterGV.WithKind(cluster.Spec.RKEConfig.InfrastructureRef.Kind).GroupKind()

	if _, ok := InfraClusterToMachineTemplateGKs[infraClusterGK]; ok {
		return true
	}

	return false
}

func (h *handler) updateNextCleanupTime(cluster *provv1.Cluster, period time.Duration, now time.Time) (*provv1.Cluster, error) {
	cluster = cluster.DeepCopy()

	annotations := cluster.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[nextCleanupAnnotationKey] = now.Add(period).UTC().Format(time.RFC3339)

	cluster.Annotations = annotations

	return h.provClusterController.Update(cluster)
}

func (h *handler) cleanup() {
	for {
		logrus.Infof("%s waiting for scheduled cleanups...", logPrefix)

		select {
		case <-h.ctx.Done():
			return
		case cluster := <-h.cleanupQueue:
			if h.isLocal(cluster) {
				// Handle objects that haven't been adopted and
				// label those that have.
				h.cleanupOrLabelObjects()
			} else {
				h.cleanupOwnedObjectsByCluster(cluster)
			}
		}
	}
}

func (h *handler) cleanupOrLabelObjects() {
	logrus.Infof("%s about to label and cleanup objects", logPrefix)

	now := time.Now()

	for infraCluster, infraMachineTemplate := range InfraClusterToMachineTemplateGKs {
		err := h.cleanupOrLabelObjectsByGK(infraCluster, false, now)
		if err != nil {
			logrus.Errorf("%s cleaning up: %v", logPrefix, err)
		}

		err = h.cleanupOrLabelObjectsByGK(infraMachineTemplate, true, now)
		if err != nil {
			logrus.Errorf("%s cleaning up: %v", logPrefix, err)
		}
	}

	logrus.Infof("%s done", logPrefix)
}

func (h *handler) cleanupOrLabelObjectsByGK(objectGK schema.GroupKind, label bool, now time.Time) error {
	logrus.Infof("%s handling %s objects", logPrefix, objectGK)

	client, err := h.clientForGK(objectGK)
	if err != nil {
		return err
	}

	var hasErr bool
	cont := ""
	for {
		objs, err := client.List(h.ctx, v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=true", CleanupEnabledLabelKey),
			Limit:         pageSize,
			Continue:      cont,
		})
		if err != nil {
			return err
		}

		for _, obj := range objs.Items {
			err := h.cleanupOrLabelObject(&obj, client, label, now)
			if err != nil {
				// Try to clean-up as many templates as possible before returning.
				logrus.Errorf("%s cleaning up %s: %v", logPrefix, objectGK, err)
				hasErr = true
			}
		}

		cont = objs.GetContinue()
		if cont == "" {
			break
		}
	}

	if hasErr {
		return fmt.Errorf("some objects had errors")
	}

	return nil
}

// If an object has no owner after a grace period, delete it. If it does
// have an owner, add a label to it so it can be more easily found
// by the the per-cluster cleanup function.
func (h *handler) cleanupOrLabelObject(
	obj *unstructured.Unstructured,
	client dynamic.NamespaceableResourceInterface,
	label bool,
	now time.Time,
) error {

	if len(obj.GetOwnerReferences()) > 0 {
		if label {
			return h.labelOwnedObject(obj, client)
		}
		return nil
	}

	// Allow some time for the object to be adopted after creation, e.g. the UI
	// might create first the object and then the provisionig cluster.
	if obj.GetCreationTimestamp().Add(templateGracePeriod).After(now) {
		return nil
	}

	uid := obj.GetUID()
	rv := obj.GetResourceVersion()

	logrus.Debugf("%s deleting %s/%s...", logPrefix, obj.GetNamespace(), obj.GetName())
	err := client.Namespace(obj.GetNamespace()).Delete(h.ctx, obj.GetName(), v1.DeleteOptions{
		Preconditions: &v1.Preconditions{
			// Avoid issues with name reuse.
			UID: &uid,

			// This would prevent a delete on a last-minute adoption (as the owner references would change).
			ResourceVersion: &rv,
		},
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}

	return nil
}

func (h *handler) labelOwnedObject(obj *unstructured.Unstructured, client dynamic.NamespaceableResourceInterface) error {
	ownerName := ""
	for _, owner := range obj.GetOwnerReferences() {
		ownerGV, err := schema.ParseGroupVersion(owner.APIVersion)
		if err != nil {
			return err
		}

		if ownerGV.Group == capi.GroupVersion.Group && owner.Kind == "Cluster" {
			ownerName = owner.Name
		}
	}

	if ownerName == "" {
		return fmt.Errorf("no capi cluster owner")
	}

	labels := obj.GetLabels()

	// By construction, the capi cluster name is the same as
	// the provisioning cluster name.
	if ownerName == labels[capr.ClusterNameLabel] {
		return nil
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	labels[capr.ClusterNameLabel] = ownerName

	obj.SetLabels(labels)

	_, err := client.Namespace(obj.GetNamespace()).Update(h.ctx, obj, v1.UpdateOptions{})
	return err
}

func (h *handler) cleanupOwnedObjectsByCluster(cluster *provv1.Cluster) {
	logrus.Infof("%s about to clean up templates for cluster %s/%s", logPrefix, cluster.Namespace, cluster.Name)

	now := time.Now()

	err := h.cleanupInfraMachineTemplates(cluster, now)
	if err != nil {
		logrus.Errorf("%s cleaning up templates: %v", logPrefix, err)
	}

	logrus.Infof("%s done", logPrefix)
}

func (h *handler) cleanupInfraMachineTemplates(cluster *provv1.Cluster, now time.Time) error {
	if cluster.Spec.RKEConfig == nil || cluster.Spec.RKEConfig.InfrastructureRef == nil {
		return fmt.Errorf("cluster had no infra cluster ref")
	}

	infraClusterGV, err := schema.ParseGroupVersion(cluster.Spec.RKEConfig.InfrastructureRef.APIVersion)
	if err != nil {
		return fmt.Errorf("cluster had an invalid infra cluster ref")
	}

	infraClusterGK := infraClusterGV.WithKind(cluster.Spec.RKEConfig.InfrastructureRef.Kind).GroupKind()

	templateGK, ok := InfraClusterToMachineTemplateGKs[infraClusterGK]
	if !ok {
		return nil
	}

	templateClient, err := h.clientForGK(templateGK)
	if err != nil {
		return err
	}

	nsedTemplateClient := templateClient.Namespace(cluster.GetNamespace())

	// Get the provisioning cluster from the apiserver to ensure we have a
	// fresh copy.
	cluster, err = h.provClusterController.Get(cluster.Namespace, cluster.Name, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting provisioning cluster: %w", err)
	}

	// Here we can use the cache for CAPI clusters since all we need from it is
	// to check the ownership chain from provisioning cluster to template, and
	// we expect there to be a single capi cluster for a provisioning cluser.
	capiCluster, err := h.capiClusterCache.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return fmt.Errorf("looking for CAPI cluster: %w", err)
	}

	foundOwner := false
	for _, owner := range capiCluster.OwnerReferences {
		ownerGV, err := schema.ParseGroupVersion(owner.APIVersion)
		if err != nil {
			return fmt.Errorf("parsing GroupVersion of CAPI cluster owner: %w", err)
		}

		if ownerGV.Group == provv1.SchemeGroupVersion.Group && owner.Kind == "Cluster" && owner.UID == cluster.UID {
			foundOwner = true
		}
	}

	if !foundOwner {
		return fmt.Errorf("CAPI cluster owner has no provisioning cluster owner")
	}

	templatesInUse, err := h.getUsedTemplatesByCluster(cluster)
	if err != nil {
		return err
	}

	var hasErr bool
	cont := ""
	for {
		templates, err := nsedTemplateClient.List(h.ctx, v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=true,%s=%s", CleanupEnabledLabelKey, capr.ClusterNameLabel, cluster.Name),
			Limit:         pageSize,
			Continue:      cont,
		})
		if err != nil {
			return fmt.Errorf("listing templates: %w", err)
		}

		for _, template := range templates.Items {
			if templatesInUse[template.GetName()] {
				logrus.Debugf("%s skipping template in use: %s/%s", logPrefix, template.GetNamespace(), template.GetName())
				continue
			}

			err := h.cleanupInfraMachineTemplate(&template, capiCluster, nsedTemplateClient, now)
			if err != nil {
				// Try to clean-up as many templates as possible before returning.
				logrus.Errorf("%s error cleaning up: %v", logPrefix, err)
				hasErr = true
			}
		}

		cont = templates.GetContinue()
		if cont == "" {
			break
		}
	}

	if hasErr {
		return fmt.Errorf("some objects had errors")
	}

	return nil
}

func (h *handler) cleanupInfraMachineTemplate(
	template *unstructured.Unstructured,
	capiCluster *capi.Cluster,
	client dynamic.ResourceInterface,
	now time.Time,
) error {
	templateOwnerRefs := template.GetOwnerReferences()
	if len(templateOwnerRefs) != 1 {
		return nil
	}

	templateOwnerRef := templateOwnerRefs[0]
	templateOwnerRefGV, err := schema.ParseGroupVersion(templateOwnerRef.APIVersion)
	if err != nil {
		return fmt.Errorf("parsing owner of %s/%s: %w", template.GetNamespace(), template.GetName(), err)
	}

	if templateOwnerRefGV.Group != capi.GroupVersion.Group ||
		templateOwnerRef.Kind != "Cluster" ||
		templateOwnerRef.UID != capiCluster.UID {

		return fmt.Errorf("unexpected owner for %s/%s", template.GetNamespace(), template.GetName())
	}

	if template.GetCreationTimestamp().Add(templateGracePeriod).After(now) {
		return nil
	}

	uid := template.GetUID()

	logrus.Debugf("%s deleting %s/%s...", logPrefix, template.GetNamespace(), template.GetName())
	err = client.Delete(h.ctx, template.GetName(), v1.DeleteOptions{
		Preconditions: &v1.Preconditions{
			UID: &uid,
		},
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting %s/%s: %w", template.GetNamespace(), template.GetName(), err)
	}

	return nil
}

func (h *handler) getUsedTemplatesByCluster(provCluster *provv1.Cluster) (map[string]bool, error) {

	var machinePools []provv1.RKEMachinePool
	if provCluster.Spec.RKEConfig != nil {
		machinePools = provCluster.Spec.RKEConfig.MachinePools
	}

	templatesInUse := make(map[string]bool)

	for _, mp := range machinePools {
		if mp.NodeConfig != nil {
			templatesInUse[mp.NodeConfig.Name] = true
		}
	}

	cont := ""
	for {
		machineDeployments, err := h.machineDeploymentClient.List(provCluster.Namespace, v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, provCluster.Name),
			Limit:         pageSize,
			Continue:      cont,
		})
		if err != nil {
			return nil, fmt.Errorf("listing machine deployments: %w", err)
		}

		for _, md := range machineDeployments.Items {
			templatesInUse[md.Spec.Template.Spec.InfrastructureRef.Name] = true
		}

		cont = machineDeployments.Continue
		if cont == "" {
			break
		}
	}

	cont = ""
	for {
		machineSets, err := h.machineSetClient.List(provCluster.Namespace, v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, provCluster.Name),
			Limit:         pageSize,
			Continue:      cont,
		})
		if err != nil {
			return nil, fmt.Errorf("listing machine sets: %w", err)
		}

		for _, ms := range machineSets.Items {
			templatesInUse[ms.Spec.Template.Spec.InfrastructureRef.Name] = true
		}

		cont = machineSets.Continue
		if cont == "" {
			break
		}
	}

	return templatesInUse, nil
}
