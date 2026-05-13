package privateregistry

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/managementuser/secret"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
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
	settings                 mgmtcontrollers.SettingController
	secrets                  corecontrollers.SecretController
	secretCache              corecontrollers.SecretCache
	project                  mgmtcontrollers.ProjectController
	projectCache             mgmtcontrollers.ProjectCache
	mgmtCluster              mgmtcontrollers.ClusterController
	mgmtClusterCache         mgmtcontrollers.ClusterCache
	provisioningClusterCache provisioningv1.ClusterCache
}

const clusterToSysProjIndex = "mgmt-system-project"

func Register(ctx context.Context, wContext *wrangler.Context) {
	h := &handler{
		settings:                 wContext.Mgmt.Setting(),
		secrets:                  wContext.Core.Secret(),
		secretCache:              wContext.Core.Secret().Cache(),
		project:                  wContext.Mgmt.Project(),
		projectCache:             wContext.Mgmt.Project().Cache(),
		mgmtCluster:              wContext.Mgmt.Cluster(),
		mgmtClusterCache:         wContext.Mgmt.Cluster().Cache(),
		provisioningClusterCache: wContext.Provisioning.Cluster().Cache(),
	}

	// maps cluster names to their system projects
	h.projectCache.AddIndexer(clusterToSysProjIndex, func(obj *v3.Project) ([]string, error) {
		if obj != nil && obj.Labels != nil && obj.ObjectMeta.Labels[project.SystemProjectLabelKey] == "true" {
			if obj.Spec.ClusterName == "" {
				return nil, fmt.Errorf("[private-registry] system project has empty cluster name")
			}
			return []string{obj.Spec.ClusterName}, nil
		}
		return []string{}, nil
	})

	h.settings.OnChange(ctx, "sync-source-secrets", h.labelSourceGlobalRegistryPullSecret)
	h.mgmtCluster.OnChange(ctx, "manage-system-project-pull-secret-label", h.labelSystemProject)
	h.mgmtCluster.OnChange(ctx, "manage-pss-downstream-clusters", h.manageImportedAndHostedClusterPSS)

	// enqueue any v3 cluster which relies on the global configuration to ensure that in place updates to global pull secrets are promptly applied to downstream clusters.
	relatedresource.WatchClusterScoped(ctx, "sync-cluster-global-pull-secret-contents", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		sec, ok := obj.(*kcorev1.Secret)
		if !ok {
			logrus.Errorf("[private-registry] failed to convert to secret")
			return nil, nil
		}
		if sec.Labels == nil || sec.Labels[util.SourcePullSecretLabel] != "true" {
			return nil, nil
		}

		configuredPullSecrets := activeGlobalPullSecrets()
		if !configuredPullSecrets.Has(sec.Name) {
			return nil, nil
		}

		return findV3ClustersUsingGlobalPullSecrets(h.mgmtClusterCache)
	}, wContext.Mgmt.Cluster(), wContext.Core.Secret())

	// enqueue any v3 cluster which relies on the global configuration to ensure that setting changes are promptly applied to downstream clusters
	relatedresource.WatchClusterScoped(ctx, "sync-cluster-global-pull-secret-settings", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if name != settings.SystemDefaultRegistryPullSecrets.Name && name != settings.SystemDefaultRegistry.Name {
			return nil, nil
		}
		return findV3ClustersUsingGlobalPullSecrets(h.mgmtClusterCache)
	}, wContext.Mgmt.Cluster(), wContext.Mgmt.Setting())
}

// labelSystemProject manages the 'management.cattle.io/use-global-private-registry-pull-secret' label
// on system projects associated with imported or hosted clusters which rely on the global system default
// registry pull secrets. This label is referenced in the project scoped secrets implementation to
// ensure that the globally defined pull secrets are properly copied into the relevant namespaces in
// the local cluster and downstream imported and hosted clusters.
func (h *handler) labelSystemProject(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	logrus.Tracef("[private-registry] syncing system project label for cluster %q", cluster.Name)
	if (!v3.ClusterConditionSystemProjectCreated.IsTrue(cluster) || !v3.ClusterConditionAgentDeployed.IsTrue(cluster)) && cluster.Name != "local" {
		logrus.Debugf("[private-registry] cluster %q not ready (systemProjectCreated=%v, agentDeployed=%v), skipping",
			cluster.Name,
			v3.ClusterConditionSystemProjectCreated.IsTrue(cluster),
			v3.ClusterConditionAgentDeployed.IsTrue(cluster))
		return cluster, nil
	}

	// Don't do this for provisioned clusters, the creds will be passed via prov2 and the system-agent.
	if !util.MgmtNameRegexp.MatchString(cluster.Name) {
		logrus.Debugf("[private-registry] cluster %q is provisioned, not imported/hosted; skipping system project label management", cluster.Name)
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
		_, err = h.project.Update(sysProj)
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
		_, err = h.project.Update(sysProj)
		if err != nil {
			logrus.Errorf("[private-registry] failed to add global pull secret label to system project %q for cluster %q: %v", sysProj.Name, cluster.Name, err)
			return cluster, err
		}
	}

	return cluster, nil
}

// labelSourceGlobalRegistryPullSecret handles the labeling and unlabeling of global system default registry pull secrets.
// The label is used by other controls to synchronize the contents of the source pull secret to PSS copies in downstream clusters and
// system project namespaces in the local cluster.
func (h *handler) labelSourceGlobalRegistryPullSecret(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || setting.Name != settings.SystemDefaultRegistryPullSecrets.Name {
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

// manageImportedAndHostedClusterPSS handles the synchronization of source image pull secrets and project scoped secrets
// for downstream imported and hosted clusters only. This handler specifically focuses on clusters which define an override,
// and ignores clusters that rely on the global configuration. Imported / Hosted clusters which rely on the GSDR will
// receive their pull secrets via alternative system project pull secret propagation logic incorporated in the PSS implementation.
func (h *handler) manageImportedAndHostedClusterPSS(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.Name == "local" {
		return cluster, nil
	}

	logrus.Tracef("[private-registry] managing PSS for cluster %q", cluster.Name)

	// don't use pull secrets if we're working with a provisioned cluster, credentials
	// will be configured at the container runtime level.
	if !util.MgmtNameRegexp.MatchString(cluster.Name) {
		logrus.Debugf("[private-registry] cluster %q is provisioned, not imported/hosted; skipping PSS management", cluster.Name)
		return cluster, nil
	}

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
	if (privateRegistry == nil || isGlobalDefault) && len(createdPSS) == 0 {
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
	logrus.Debugf("[private-registry] resolved %d source pull secrets and found %d existing PSS(s) for cluster %q", len(sourceAuthSecrets), len(createdPSS), cluster.Name)

	// Delete any PSS's that were created for secrets no longer specified on the cluster object.
	for _, pss := range createdPSS {
		if slices.ContainsFunc(sourceAuthSecrets, func(s *kcorev1.Secret) bool { return s.Name == pss.Name }) {
			continue
		}
		logrus.Debugf("[private-registry] deleting stale PSS %q from namespace %q", pss.Name, backingNamespace)
		err = h.secrets.Delete(pss.Namespace, pss.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logrus.Errorf("[private-registry] failed to delete stale PSS %q from namespace %q: %v", pss.Name, backingNamespace, err)
			return cluster, err
		}
	}

	if isGlobalDefault || privateRegistry == nil {
		return cluster, nil
	}

	// Create any missing PSS's
	for _, sourcePullSecret := range sourceAuthSecrets {
		data, err := util.ConvertToDockerConfigJson(privateRegistry.URL, sourcePullSecret)
		if err != nil {
			logrus.Errorf("[private-registry] failed to convert pull secret %q to docker config for cluster %q: %v", sourcePullSecret.Name, cluster.Name, err)
			continue
		}

		existingSecret := findSecretByName(createdPSS, sourcePullSecret.Name)
		if existingSecret == nil {
			logrus.Debugf("[private-registry] creating PSS %q in namespace %q for cluster %q", sourcePullSecret.Name, backingNamespace, cluster.Name)
			pss := buildPSS(sysProj, backingNamespace, sourcePullSecret.Name, data)
			_, err = h.secrets.Create(pss)
			if err != nil && !errors.IsAlreadyExists(err) {
				logrus.Errorf("[private-registry] failed to create PSS %q in namespace %q: %v", sourcePullSecret.Name, backingNamespace, err)
				return cluster, err
			}
			continue
		}

		existingData := existingSecret.Data[kcorev1.DockerConfigJsonKey]
		if bytes.Equal(existingData, data) {
			logrus.Tracef("[private-registry] PSS %q in namespace %q is up to date", sourcePullSecret.Name, backingNamespace)
			continue
		}

		logrus.Debugf("[private-registry] updating PSS %q in namespace %q for cluster %q", sourcePullSecret.Name, backingNamespace, cluster.Name)
		existingSecret = existingSecret.DeepCopy()
		existingSecret.Data[kcorev1.DockerConfigJsonKey] = data
		_, err = h.secrets.Update(existingSecret)
		if err != nil {
			logrus.Errorf("[private-registry] failed to update PSS %q in namespace %q: %v", sourcePullSecret.Name, backingNamespace, err)
			return cluster, err
		}
	}

	return cluster, nil
}

func (h *handler) getSystemProjectForCluster(clusterName string) (*v3.Project, error) {
	logrus.Tracef("[private-registry] looking up system project for cluster %q", clusterName)
	clusterSystemProj, err := h.projectCache.GetByIndex(clusterToSysProjIndex, clusterName)
	if err != nil {
		logrus.Errorf("[private-registry] failed to look up system project for cluster %q: %v", clusterName, err)
		return nil, err
	}
	if clusterSystemProj == nil || len(clusterSystemProj) == 0 {
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
				util.CopiedPullSecretLabel:                    "true",
				secret.ProjectScopedSecretLabel:               proj.Name,
				"management.cattle.io/registry-scoped-secret": "true",
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

func findV3ClustersUsingGlobalPullSecrets(clusterCache mgmtcontrollers.ClusterCache) ([]relatedresource.Key, error) {
	clusters, err := clusterCache.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var clusterNames []relatedresource.Key
	for _, cluster := range clusters {
		_, isGlobalDefault := util.GetPrivateRegistry(cluster)
		if !isGlobalDefault {
			continue
		}
		if util.MgmtNameRegexp.MatchString(cluster.Name) {
			clusterNames = append(clusterNames, relatedresource.Key{
				Name: cluster.Name,
			})
		}
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
