package privateregistry

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/managementuser/secret"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

type handler struct {
	secrets          corecontrollers.SecretController
	secretCache      corecontrollers.SecretCache
	projects         mgmtcontrollers.ProjectController
	projectCache     mgmtcontrollers.ProjectCache
	mgmtClusters     mgmtcontrollers.ClusterController
	mgmtClusterCache mgmtcontrollers.ClusterCache
}

const (
	clusterToSysProjIndex       = "mgmt-system-project"
	clusterEnqueueBatchSize     = 10
	clusterEnqueueDelay         = 2 * time.Second
	clusterEnqueueJitterCeiling = 750 // milliseconds
	// registrySecretUIHint is a label used by the UI to conditionally render special
	// fields specific to registry credentials when viewed in the UI.
	registrySecretUIHint = "management.cattle.io/registry-scoped-secret"
)

func Register(ctx context.Context, wContext *wrangler.Context) {
	h := &handler{
		secrets:          wContext.Core.Secret(),
		secretCache:      wContext.Core.Secret().Cache(),
		projects:         wContext.Mgmt.Project(),
		projectCache:     wContext.Mgmt.Project().Cache(),
		mgmtClusters:     wContext.Mgmt.Cluster(),
		mgmtClusterCache: wContext.Mgmt.Cluster().Cache(),
	}

	// maps cluster names to their system projects
	h.projectCache.AddIndexer(clusterToSysProjIndex, func(obj *v3.Project) ([]string, error) {
		if obj != nil && obj.Labels != nil && obj.Labels[project.SystemProjectLabelKey] == "true" {
			if obj.Spec.ClusterName == "" {
				return nil, fmt.Errorf("[private-registry] system project has empty cluster name")
			}
			return []string{obj.Spec.ClusterName}, nil
		}
		return []string{}, nil
	})

	// Updates source pull secrets in the cattle-system namespace when the global settings change
	wContext.Mgmt.Setting().OnChange(ctx, "sync-source-secrets", h.labelConfiguredSourceGlobalRegistryPullSecrets)

	// Labels the system project of imported or hosted clusters to request image pull secrets
	wContext.Mgmt.Cluster().OnChange(ctx, "manage-configured-system-project-pull-secret-label", h.labelSystemProject)

	// Creates PSS copies of the cluster level pull secrets configured for imported and hosted clusters.
	wContext.Mgmt.Cluster().OnChange(ctx, "manage-pss-downstream-clusters", h.manageClusterSpecificPSS)

	// Ensures that secrets configured in the global settings always have the source pull secret label.
	wContext.Core.Secret().OnChange(ctx, "manage-configured-system-project-pull-secret-label", h.labelSourceGlobalRegistryPullSecret)

	// enqueue any v3 cluster which relies on the global configuration to ensure that in place updates to global pull secrets are promptly applied to downstream clusters.
	relatedresource.WatchClusterScoped(ctx, "sync-cluster-global-pull-secret-contents", h.syncClusterOnGlobalPullSecretChange, wContext.Mgmt.Cluster(), wContext.Core.Secret())

	// enqueue any v3 cluster which relies on the global configuration to ensure that setting changes are promptly applied to downstream clusters
	relatedresource.WatchClusterScoped(ctx, "sync-cluster-global-pull-secret-settings", h.syncClusterOnGlobalRegistrySettingChange, wContext.Mgmt.Cluster(), wContext.Mgmt.Setting())
}

// labelSystemProject manages the 'management.cattle.io/use-global-private-registry-pull-secret' label
// on system projects associated with imported or hosted clusters which rely on the global system default
// registry pull secrets. This label is referenced in the project scoped secrets implementation to
// ensure that the globally defined pull secrets are properly copied into the relevant namespaces in
// downstream imported and hosted clusters.
func (h *handler) labelSystemProject(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.Name == "local" {
		return cluster, nil
	}

	// Don't do this for provisioned clusters, the creds will be passed via prov2 and the system-agent.
	if !util.MgmtNameRegexp.MatchString(cluster.Name) {
		logrus.Tracef("[private-registry] cluster %q is provisioned, not imported/hosted; skipping system project label management", cluster.Name)
		return cluster, nil
	}

	logrus.Tracef("[private-registry] syncing system project label for cluster %q", cluster.Name)
	if !v3.ClusterConditionSystemProjectCreated.IsTrue(cluster) || !v3.ClusterConditionAgentDeployed.IsTrue(cluster) {
		logrus.Debugf("[private-registry] cluster %q not ready (systemProjectCreated=%v, agentDeployed=%v), skipping",
			cluster.Name,
			v3.ClusterConditionSystemProjectCreated.IsTrue(cluster),
			v3.ClusterConditionAgentDeployed.IsTrue(cluster))
		return cluster, nil
	}

	sysProj, err := h.getSystemProjectForCluster(cluster.Name)
	if err != nil {
		logrus.Errorf("[private-registry] failed to get system project for cluster %q: %v", cluster.Name, err)
		return cluster, err
	}

	usesGlobalSecrets := sysProj.Labels != nil && sysProj.Labels[secret.NeedsGlobalPrivateRegistryPullSecret] == "true"
	registry, isGlobalDefault := util.GetPrivateRegistry(cluster)

	logrus.Tracef("[private-registry] cluster %q: isGlobalDefault=%v, usesGlobalSecrets=%v, registryNil=%v",
		cluster.Name, isGlobalDefault, usesGlobalSecrets, registry == nil)

	shouldUseGlobalSecrets := isGlobalDefault && registry != nil && len(registry.PullSecrets) > 0

	if usesGlobalSecrets && !shouldUseGlobalSecrets {
		logrus.Debugf("[private-registry] removing global pull secret label from system project %q for cluster %q", sysProj.Name, cluster.Name)
		sysProj = sysProj.DeepCopy()
		delete(sysProj.Labels, secret.NeedsGlobalPrivateRegistryPullSecret)
		_, err = h.projects.Update(sysProj)
		if err != nil {
			logrus.Errorf("[private-registry] failed to remove global pull secret label from system project %q for cluster %q: %v", sysProj.Name, cluster.Name, err)
			return cluster, err
		}
		return cluster, nil
	}

	if !usesGlobalSecrets && shouldUseGlobalSecrets {
		logrus.Debugf("[private-registry] adding global pull secret label to system project %q for cluster %q", sysProj.Name, cluster.Name)
		sysProj = sysProj.DeepCopy()
		if sysProj.Labels == nil {
			sysProj.Labels = map[string]string{}
		}
		sysProj.Labels[secret.NeedsGlobalPrivateRegistryPullSecret] = "true"
		_, err = h.projects.Update(sysProj)
		if err != nil {
			logrus.Errorf("[private-registry] failed to add global pull secret label to system project %q for cluster %q: %v", sysProj.Name, cluster.Name, err)
			return cluster, err
		}
	}

	return cluster, nil
}

// labelConfiguredSourceGlobalRegistryPullSecrets handles the labeling and unlabeling of global system default registry pull secrets whenever the
// global settings change. The label is used by other controllers to synchronize the contents of the source pull secret to PSS copies
// in downstream clusters and system project namespaces in the local cluster.
func (h *handler) labelConfiguredSourceGlobalRegistryPullSecrets(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || (setting.Name != settings.SystemDefaultRegistryPullSecrets.Name && setting.Name != settings.SystemDefaultRegistry.Name) {
		return setting, nil
	}

	logrus.Tracef("[private-registry] syncing global registry pull secret labels for setting %q", setting.Name)

	existingGlobalPullSecrets, err := h.secretCache.List(namespaces.System, labels.SelectorFromSet(map[string]string{
		util.SourcePullSecretLabel: "true",
	}))
	if err != nil {
		logrus.Errorf("[private-registry] failed to list labeled source pull secrets in namespace %q: %v", namespaces.System, err)
		return setting, err
	}

	existingSecretsSet := sets.New[string]()
	for _, s := range existingGlobalPullSecrets {
		existingSecretsSet.Insert(s.Name)
	}
	logrus.Debugf("[private-registry] found %d existing labeled pull secret(s): %v", existingSecretsSet.Len(), existingSecretsSet.UnsortedList())

	specifiedSecretsSet := activeGlobalPullSecrets()

	logrus.Debugf("[private-registry] setting specifies %d pull secret(s): %v", specifiedSecretsSet.Len(), specifiedSecretsSet.UnsortedList())

	toRemove := existingSecretsSet.Difference(specifiedSecretsSet).UnsortedList()
	if len(toRemove) > 0 {
		logrus.Debugf("[private-registry] removing source label from %d secret(s): %v", len(toRemove), toRemove)
	}

	for _, s := range toRemove {
		sec, err := h.secretCache.Get(namespaces.System, s)
		if err != nil {
			logrus.Errorf("[private-registry] failed to get secret %q in namespace %q: %v", s, namespaces.System, err)
			return setting, err
		}
		sec = sec.DeepCopy()
		delete(sec.Labels, util.SourcePullSecretLabel)
		delete(sec.Annotations, secret.PSSIgnoreNamespacesAnnotation)
		logrus.Tracef("[private-registry] removing source label and PSS annotation from secret %q", s)
		_, err = h.secrets.Update(sec)
		if err != nil {
			logrus.Errorf("[private-registry] failed to update secret %q in namespace %q: %v", s, namespaces.System, err)
			return setting, err
		}
	}

	toLabel := specifiedSecretsSet.Difference(existingSecretsSet).UnsortedList()
	if len(toLabel) > 0 {
		logrus.Debugf("[private-registry] applying source label to %d secret(s): %v", len(toLabel), toLabel)
	}

	for _, s := range toLabel {
		sec, err := h.secretCache.Get(namespaces.System, s)
		if err != nil {
			logrus.Errorf("[private-registry] failed to get secret %q in namespace %q: %v", s, namespaces.System, err)
			return setting, err
		}
		sec = sec.DeepCopy()
		if sec.Labels == nil {
			sec.Labels = map[string]string{}
		}
		sec.Labels[util.SourcePullSecretLabel] = "true"
		if sec.Annotations == nil {
			sec.Annotations = map[string]string{}
		}
		sec.Annotations[secret.PSSIgnoreNamespacesAnnotation] = strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ",")
		logrus.Tracef("[private-registry] applying source label and PSS annotation to secret %q", s)
		_, err = h.secrets.Update(sec)
		if err != nil {
			logrus.Errorf("[private-registry] failed to update secret %q in namespace %q: %v", s, namespaces.System, err)
			return setting, err
		}
	}

	return setting, nil
}

// labelSourceGlobalRegistryPullSecret ensures that the secrets defined in the SystemDefaultRegistryPullSecrets global setting
// always have the correct SourcePullSecretLabel label and PSS namespace ignore label values.
func (h *handler) labelSourceGlobalRegistryPullSecret(_ string, s *kcorev1.Secret) (*corev1.Secret, error) {
	if s == nil || s.DeletionTimestamp != nil || s.Namespace != namespaces.System || s.Data == nil {
		return s, nil
	}

	activePullSecrets := activeGlobalPullSecrets()
	if !activePullSecrets.Has(s.Name) {
		return s, nil
	}

	sps, hasSps := s.Labels[util.SourcePullSecretLabel]
	ins, hasIns := s.Annotations[secret.PSSIgnoreNamespacesAnnotation]
	expectedIns := strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ",")
	if !hasSps || sps != "true" || !hasIns || ins == "" || ins != expectedIns {
		s = s.DeepCopy()
		if s.Labels == nil {
			s.Labels = map[string]string{}
		}
		if s.Annotations == nil {
			s.Annotations = map[string]string{}
		}
		if !hasSps || sps != "true" {
			s.Labels[util.SourcePullSecretLabel] = "true"
		}
		if !hasIns || ins == "" || ins != expectedIns {
			s.Annotations[secret.PSSIgnoreNamespacesAnnotation] = expectedIns
		}
		_, err := h.secrets.Update(s)
		if err != nil {
			logrus.Errorf("[private-registry] failed to label secret %q as source pull secret: %v", s.Name, err)
			return s, err
		}
	}

	return s, nil
}

// manageClusterSpecificPSS handles the synchronization of source image pull secrets and project scoped secrets
// for downstream imported and hosted clusters, as well as the local cluster. This handler specifically
// focuses on clusters which define an override, and ignores clusters that rely on the global configuration,
// with an exception for the local cluster. Imported / Hosted clusters which rely on the GSDR will receive
// their pull secrets via alternative system project pull secret propagation logic incorporated in the PSS implementation.
func (h *handler) manageClusterSpecificPSS(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	logrus.Tracef("[private-registry] managing PSS for cluster %q", cluster.Name)

	// don't use pull secrets if we're working with a provisioned cluster, credentials
	// will be configured at the container runtime level.
	if !util.MgmtNameRegexp.MatchString(cluster.Name) {
		logrus.Debugf("[private-registry] cluster %q is provisioned, not imported/hosted; skipping PSS management", cluster.Name)
		return cluster, nil
	}

	if cluster.Name != "local" && (!v3.ClusterConditionSystemProjectCreated.IsTrue(cluster) || !v3.ClusterConditionAgentDeployed.IsTrue(cluster)) {
		logrus.Debugf("[private-registry] cluster %q not ready (systemProjectCreated=%v, agentDeployed=%v), skipping",
			cluster.Name,
			v3.ClusterConditionSystemProjectCreated.IsTrue(cluster),
			v3.ClusterConditionAgentDeployed.IsTrue(cluster))
		return cluster, nil
	}

	sysProj, err := h.getSystemProjectForCluster(cluster.Name)
	if err != nil {
		logrus.Errorf("[private-registry] failed to get system project for cluster %q: %v", cluster.Name, err)
		return cluster, err
	}

	backingNamespace := sysProj.GetProjectBackingNamespace()
	logrus.Tracef("[private-registry] using backing namespace %q for cluster %q", backingNamespace, cluster.Name)

	// gather all PSS's for this specific cluster in its system projects backing namespace
	createdPSS, err := h.secretCache.List(backingNamespace, labels.SelectorFromSet(map[string]string{
		util.CopiedPullSecretLabel: "true",
	}))
	if err != nil {
		logrus.Errorf("[private-registry] failed to list copied PSS(s) in backing namespace %q for cluster %q: %v", backingNamespace, cluster.Name, err)
		return cluster, err
	}

	privateRegistry, isGlobalDefault := util.GetPrivateRegistry(cluster)
	if cluster.Name != "local" && (privateRegistry == nil || isGlobalDefault) && len(createdPSS) == 0 {
		logrus.Debugf("[private-registry] no applicable cluster-level pull secrets for cluster %q (isGlobalDefault=%v), skipping", cluster.Name, isGlobalDefault)
		return cluster, nil
	}

	var configuredPullSecrets []kcorev1.SecretReference
	if privateRegistry != nil {
		configuredPullSecrets = privateRegistry.PullSecrets
	}

	// This finds the secrets that are specified
	// on the cluster and creates a slice of all the secrets
	// which currently exist.
	var sourceAuthSecrets []*kcorev1.Secret
	for _, ds := range configuredPullSecrets {
		s, err := h.secretCache.Get(ds.Namespace, ds.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Warnf("[private-registry] pull secret %q/%q defined on cluster %q not found, skipping", ds.Namespace, ds.Name, cluster.Name)
				continue
			}
			logrus.Errorf("[private-registry] failed to get pull secret %q/%q for cluster %q: %v", ds.Namespace, ds.Name, cluster.Name, err)
			return cluster, err
		}
		logrus.Tracef("[private-registry] resolved source pull secret %q/%q for cluster %q", ds.Namespace, ds.Name, cluster.Name)
		sourceAuthSecrets = append(sourceAuthSecrets, s)
	}
	logrus.Tracef("[private-registry] resolved %d source pull secrets and found %d existing PSS(s) for cluster %q", len(sourceAuthSecrets), len(createdPSS), cluster.Name)

	// Delete any PSS's that were created for secrets no longer specified on the cluster object.
	for _, pss := range createdPSS {
		if slices.ContainsFunc(sourceAuthSecrets, func(s *kcorev1.Secret) bool {
			expectedName := s.Name
			if cluster.Name != "local" {
				expectedName = util.GeneratePullSecretName(s.Name)
			}
			return expectedName == pss.Name
		}) {
			continue
		}
		logrus.Debugf("[private-registry] deleting stale PSS %q from namespace %q", pss.Name, backingNamespace)
		err = h.secrets.Delete(pss.Namespace, pss.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logrus.Errorf("[private-registry] failed to delete stale PSS %q from namespace %q: %v", pss.Name, backingNamespace, err)
			return cluster, err
		}
	}

	if (cluster.Name != "local" && isGlobalDefault) || privateRegistry == nil {
		return cluster, nil
	}

	// Create any missing PSS's
	for _, sourcePullSecret := range sourceAuthSecrets {
		data, err := util.ConvertToDockerConfigJson(privateRegistry.URL, sourcePullSecret)
		if err != nil {
			logrus.Errorf("[private-registry] failed to convert pull secret %q to docker config for cluster %q: %v", sourcePullSecret.Name, cluster.Name, err)
			return cluster, err
		}

		pssName := sourcePullSecret.Name
		if cluster.Name != "local" {
			pssName = util.GeneratePullSecretName(pssName)
		}

		existingSecret := findSecretByName(createdPSS, pssName)
		if existingSecret == nil {
			logrus.Debugf("[private-registry] creating PSS %q in namespace %q for cluster %q", pssName, backingNamespace, cluster.Name)
			pss := buildPSS(sysProj, backingNamespace, pssName, data)
			_, err := h.secrets.Create(pss)
			if err == nil {
				continue
			}
			if !errors.IsAlreadyExists(err) {
				logrus.Errorf("[private-registry] failed to create PSS %q in namespace %q: %v", pssName, backingNamespace, err)
				return cluster, err
			}
			// adopt the pull secret if it already exists but does not have
			// the expected label. We fetch from cache to avoid relying on the
			// potentially nil return value from Create on conflict.
			existing, err := h.secretCache.Get(backingNamespace, pssName)
			if err != nil {
				logrus.Errorf("[private-registry] failed to get existing PSS %q in namespace %q for adoption: %v", pssName, backingNamespace, err)
				return cluster, err
			}
			existing = existing.DeepCopy()
			if existing.Labels == nil {
				existing.Labels = map[string]string{}
			}
			if existing.Data == nil {
				existing.Data = map[string][]byte{}
			}
			existing.Labels[util.CopiedPullSecretLabel] = "true"
			existing.Labels[registrySecretUIHint] = "true"
			existing.Data[kcorev1.DockerConfigJsonKey] = data
			logrus.Debugf("[private-registry] adopting existing PSS %q in namespace %q for cluster %q", pssName, backingNamespace, cluster.Name)
			_, err = h.secrets.Update(existing)
			if err != nil {
				logrus.Errorf("[private-registry] failed to update adopted PSS %q in namespace %q: %v", pssName, backingNamespace, err)
				return cluster, err
			}
			continue
		}

		if existingSecret.Data == nil {
			existingSecret.Data = map[string][]byte{}
		}

		registrySecretValue, isRegistrySecret := existingSecret.Labels[registrySecretUIHint]
		existingData := existingSecret.Data[kcorev1.DockerConfigJsonKey]
		if bytes.Equal(existingData, data) && isRegistrySecret && registrySecretValue == "true" {
			logrus.Tracef("[private-registry] PSS %q in namespace %q is up to date", pssName, backingNamespace)
			continue
		}

		logrus.Debugf("[private-registry] updating PSS %q in namespace %q for cluster %q", pssName, backingNamespace, cluster.Name)
		existingSecret = existingSecret.DeepCopy()
		existingSecret.Data[kcorev1.DockerConfigJsonKey] = data
		existingSecret.Labels[registrySecretUIHint] = "true"
		_, err = h.secrets.Update(existingSecret)
		if err != nil {
			logrus.Errorf("[private-registry] failed to update PSS %q in namespace %q: %v", pssName, backingNamespace, err)
			return cluster, err
		}
	}

	return cluster, nil
}

// syncClusterOnGlobalPullSecretChange re-enqueues all imported/hosted/local clusters that rely on the
// global registry configuration whenever a source pull secret in cattle-system is updated.
func (h *handler) syncClusterOnGlobalPullSecretChange(_ string, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	sec, ok := obj.(*kcorev1.Secret)
	if !ok {
		return nil, nil
	}
	if sec.Labels == nil || sec.Labels[util.SourcePullSecretLabel] != "true" {
		return nil, nil
	}

	configuredPullSecrets := activeGlobalPullSecrets()
	if !configuredPullSecrets.Has(sec.Name) {
		return nil, nil
	}

	clusters, err := findV3ClustersUsingGlobalPullSecrets(h.mgmtClusterCache)
	if err != nil {
		return nil, err
	}
	enqueueStaggered(clusters, h.mgmtClusters)
	return nil, nil
}

// syncClusterOnGlobalRegistrySettingChange re-enqueues all imported/hosted/local clusters that rely on
// the global registry configuration whenever the SystemDefaultRegistry or SystemDefaultRegistryPullSecrets
// settings are updated.
func (h *handler) syncClusterOnGlobalRegistrySettingChange(_ string, name string, _ runtime.Object) ([]relatedresource.Key, error) {
	if name != settings.SystemDefaultRegistryPullSecrets.Name && name != settings.SystemDefaultRegistry.Name {
		return nil, nil
	}
	clusters, err := findV3ClustersUsingGlobalPullSecrets(h.mgmtClusterCache)
	if err != nil {
		return nil, err
	}
	enqueueStaggered(clusters, h.mgmtClusters)
	return nil, nil
}

func (h *handler) getSystemProjectForCluster(clusterName string) (*v3.Project, error) {
	logrus.Tracef("[private-registry] looking up system project for cluster %q", clusterName)
	clusterSystemProj, err := h.projectCache.GetByIndex(clusterToSysProjIndex, clusterName)
	if err != nil {
		logrus.Errorf("[private-registry] failed to look up system project for cluster %q: %v", clusterName, err)
		return nil, err
	}
	if len(clusterSystemProj) == 0 {
		logrus.Tracef("[private-registry] no system project found for cluster %q", clusterName)
		return nil, fmt.Errorf("no system project found for cluster %q", clusterName)
	}
	localSystemProj := clusterSystemProj[0]
	logrus.Tracef("[private-registry] found system project %q for cluster %q", localSystemProj.Name, clusterName)
	return localSystemProj, nil
}

func buildPSS(proj *v3.Project, systemProjectBackingNamespace, name string, dockerconfigjson []byte) *kcorev1.Secret {
	pss := &kcorev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: systemProjectBackingNamespace,
			Annotations: map[string]string{
				secret.PSSIgnoreNamespacesAnnotation: strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ","),
			},
			Labels: map[string]string{
				util.CopiedPullSecretLabel:      "true",
				secret.ProjectScopedSecretLabel: proj.Name,
				registrySecretUIHint:            "true",
			},
		},
		Data: map[string][]byte{
			kcorev1.DockerConfigJsonKey: dockerconfigjson,
		},
		Type: kcorev1.SecretTypeDockerConfigJson,
	}

	return pss
}

// findSecretByName returns the first secret in the slice whose name matches the given name, or nil if not found.
func findSecretByName(secrets []*kcorev1.Secret, name string) *kcorev1.Secret {
	for _, s := range secrets {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func findV3ClustersUsingGlobalPullSecrets(clusterCache mgmtcontrollers.ClusterCache) ([]string, error) {
	clusters, err := clusterCache.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var clusterNames []string
	for _, cluster := range clusters {
		_, isGlobalDefault := util.GetPrivateRegistry(cluster)
		if !isGlobalDefault {
			continue
		}
		clusterNames = append(clusterNames, cluster.Name)
	}
	return clusterNames, nil
}

func activeGlobalPullSecrets() sets.Set[string] {
	specifiedSecretsSet := sets.New[string]()
	registry, _ := util.GetPrivateRegistry(nil)
	if registry != nil {
		for _, p := range registry.PullSecrets {
			specifiedSecretsSet.Insert(p.Name)
		}
	}
	return specifiedSecretsSet
}

// enqueueStaggered enqueues the given cluster names with a staggered delay to avoid thundering herd issues.
// Clusters are enqueued in batches of 10 with a base delay of 2 seconds between batches, plus a random jitter
// of up to 750 milliseconds.
func enqueueStaggered(clusterNames []string, clusterController mgmtcontrollers.ClusterController) {
	for i, cluster := range clusterNames {
		batchIndex := i / clusterEnqueueBatchSize
		jitter := time.Duration(rand.IntN(clusterEnqueueJitterCeiling)) * time.Millisecond
		delay := time.Duration(batchIndex)*(clusterEnqueueDelay) + jitter
		clusterController.EnqueueAfter(cluster, delay)
	}
}
