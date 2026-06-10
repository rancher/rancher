package helmop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/cluster"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/name"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"
)

// helmOpManagedPullSecretLabel is applied to pull secrets created in a chart's release namespace
// by managePullSecrets, to allow them to be identified and garbage collected when no longer needed.
const helmOpManagedPullSecretLabel = "management.cattle.io/helm-op-pull-secret"

// helmOpReleaseLabelKey is applied alongside helmOpManagedPullSecretLabel to scope managed
// pull secrets to the specific Helm release that created them, preventing cross-release cleanup.
const helmOpReleaseLabelKey = "management.cattle.io/helm-op-release"

// createNamespaceAndPullSecrets is responsible for creating the release namespace for the charts on install/upgrade operations and managing the default image pull
// secrets to be used by the charts. It checks if each chart supports image pull secrets and that it has a system-default-registry configured. If so,
// it creates/updates the pull secrets in the release namespace and injects them in the charts values.yaml if the user has not already configured them. The management
// of pull secrets may also be skipped if the provided ctx includes a valid apiRequest, and that request specifies the 'skipPullSecrets' query parameter as 'true'.
func (s *Operations) createNamespaceAndPullSecrets(ctx context.Context, status catalog.OperationStatus, cmds Commands, clusterRepoName string) error {
	ns, err := s.createNamespace(ctx, status.Namespace, status.ProjectID)
	if err != nil {
		return err
	}

	apiRequest := types.GetAPIContext(ctx)
	if clusterRepoName != "rancher-charts" || (apiRequest != nil && apiRequest.Query.Get("skipPullSecrets") == "true") {
		return nil
	}

	repo, err := s.clusterReposCache.Get(clusterRepoName)
	if err != nil {
		log.Errorf("[helmop] createNamespaceAndPullSecrets: failed to get cluster repo %q from cache: %v", clusterRepoName, err)
		return err
	}

	for i := range cmds {
		if cmds[i].ReleaseName == "" {
			continue
		}

		var baseValues, values map[string]any
		if err := yaml.Unmarshal(cmds[i].ChartBaseValues, &baseValues); err != nil {
			log.Errorf("[helmop] createNamespaceAndPullSecrets: failed to unmarshal chart base values: %v", err)
			return err
		}

		if err := yaml.Unmarshal(cmds[i].Values, &values); err != nil {
			log.Errorf("[helmop] createNamespaceAndPullSecrets: failed to unmarshal chart values: %v", err)
			return err
		}

		if !s.chartSupportsImagePullSecrets(baseValues) {
			continue
		}

		currentManagedSecrets := buildManagedSecretNames(cmds[i].ReleaseName, repo.Spec.DefaultImagePullSecrets)
		existingManagedSecrets, err := s.listExistingManagedSecretNames(ns.Name, cmds[i].ReleaseName)
		if err != nil {
			return err
		}
		allManagedSecrets := append(currentManagedSecrets, existingManagedSecrets...)

		if s.chartHasOnlyUserConfiguredPullSecrets(baseValues, values, allManagedSecrets) {
			// if the user is manually specifying image pull secret names in all supported paths,
			// clean any managed ones up.
			if err := s.deleteStaleHelmOpSecrets([]string{}, existingManagedSecrets, ns.Name, cmds[i].ReleaseName); err != nil {
				return err
			}
			continue
		}

		// Create, update, or delete managed secret names in the charts release namespace.
		pullSecrets, err := s.managePullSecrets(repo, s.configuredSystemDefaultRegistry(values), ns.Name, cmds[i].ReleaseName, existingManagedSecrets)
		if err != nil {
			return err
		}

		// Ensure that the chart has the image pull secret fields correctly configured.
		log.Tracef("[helmop] injecting pull secrets into command %d (release=%q, namespace=%q, valuesLen=%d, chartBaseValuesLen=%d)", i, cmds[i].ReleaseName, cmds[i].ReleaseNamespace, len(cmds[i].Values), len(cmds[i].ChartBaseValues))
		cmds[i].Values, err = s.injectPullSecrets(cmds[i].ReleaseName, baseValues, values, pullSecrets, allManagedSecrets)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Operations) configuredSystemDefaultRegistry(values map[string]any) string {
	v, defined := getValueAtPath(values, "global", "cattle", "systemDefaultRegistry")
	if !defined {
		return ""
	}
	sdr, _ := v.(string)
	return sdr
}

func (s *Operations) chartSupportsImagePullSecrets(baseValues map[string]any) bool {
	for _, path := range imagePullSecretPaths {
		pathStr := strings.Join(path, ".")
		if _, declared := getValueAtPath(baseValues, path...); !declared {
			continue
		}
		log.Tracef("[helmop] injectPullSecrets: path %q is declared in chart base values", pathStr)
		return true
	}
	return false
}

// buildManagedSecretNames returns the names that managePullSecrets would assign to the
// given cluster repo secrets when scoped to releaseName.
func buildManagedSecretNames(releaseName string, secretRefs []catalog.SecretReference) []string {
	names := make([]string, 0, len(secretRefs))
	for _, ref := range secretRefs {
		names = append(names, name.SafeConcatName(releaseName, ref.Name))
	}
	return names
}

// chartHasOnlyUserConfiguredPullSecrets returns true when every imagePullSecrets path
// supported by the chart (present in 'baseValues') is already populated in 'values' with
// secrets that are not Rancher-managed (i.e. none of the configured secret names appear
// in managedSecretNames). In that case the user owns all pull secret configuration and
// Rancher should not inject or manage pull secrets for this chart.
func (*Operations) chartHasOnlyUserConfiguredPullSecrets(baseValues, values map[string]any, managedSecretNames []string) bool {
	managed := make(map[string]struct{}, len(managedSecretNames))
	for _, n := range managedSecretNames {
		managed[n] = struct{}{}
	}

	chartSupportsPullSecrets := false
	for _, path := range imagePullSecretPaths {
		if _, declared := getValueAtPath(baseValues, path...); !declared {
			continue
		}
		chartSupportsPullSecrets = true

		// find the pull secrets configured at one of
		// the known paths
		secrets, _ := getValueAtPath(values, path...)
		secretList, ok := secrets.([]any)
		if !ok || len(secretList) == 0 {
			return false
		}

		// determine if any of the configured secrets are managed by Rancher.
		// If not, then the user fully owns the pull secret configuration for this path,
		// and we should not inject or manage pull secrets for this chart.
		for _, entry := range secretList {
			switch v := entry.(type) {
			case string:
				if _, isManaged := managed[v]; isManaged {
					return false
				}
			case map[string]any:
				if n, ok := v["name"].(string); ok {
					if _, isManaged := managed[n]; isManaged {
						return false
					}
				}
			}
		}
		log.Tracef("[helmop] chartHasOnlyUserConfiguredPullSecrets: path %q has user-configured secrets", strings.Join(path, "."))
	}

	return chartSupportsPullSecrets
}

// listExistingManagedSecretNames returns the names of secrets in the release namespace
// that were previously created by managePullSecrets (identified by the managed labels).
func (s *Operations) listExistingManagedSecretNames(namespace, releaseName string) ([]string, error) {
	secrets, err := s.secretCache.List(namespace, labels.SelectorFromSet(labels.Set{
		helmOpManagedPullSecretLabel: "true",
		helmOpReleaseLabelKey:        releaseName,
	}))
	if err != nil {
		log.Errorf("[helmop] listExistingManagedSecretNames: failed to list managed secrets in namespace %q for release %q: %v", namespace, releaseName, err)
		return nil, err
	}
	names := make([]string, 0, len(secrets))
	for _, sec := range secrets {
		names = append(names, sec.Name)
	}
	return names, nil
}

func (s *Operations) injectPullSecrets(releaseName string, baseValues map[string]any, confValues map[string]any, secretNames []string, knownManagedNames []string) ([]byte, error) {
	known := make(map[string]struct{}, len(knownManagedNames))
	for _, n := range knownManagedNames {
		known[n] = struct{}{}
	}

	pullSecrets := make([]any, len(secretNames))
	for i, n := range secretNames {
		pullSecrets[i] = map[string]string{"name": n}
	}

	for _, path := range imagePullSecretPaths {
		if _, declared := getValueAtPath(baseValues, path...); !declared {
			continue
		}

		v, _ := getValueAtPath(confValues, path...)
		configuredPullSecrets, _ := v.([]any)
		userConfiguredSecrets, containsRancherManagedSecret := extractUserPullSecrets(configuredPullSecrets, known)

		if !containsRancherManagedSecret && len(configuredPullSecrets) > 0 {
			continue // user fully owns this path
		}
		if !containsRancherManagedSecret && len(secretNames) == 0 {
			continue // nothing existed, nothing to inject
		}

		newList := append(userConfiguredSecrets, pullSecrets...)
		log.Tracef("[helmop] injectPullSecrets: writing %d pull secrets at path %q for release %q (preserving %d user entries)", len(pullSecrets), strings.Join(path, "."), releaseName, len(userConfiguredSecrets))
		setValueAtPath(confValues, newList, path...)
	}

	result, err := json.Marshal(confValues)
	if err != nil {
		log.Errorf("[helmop] injectPullSecrets: failed to marshal modified values for release %q: %v", releaseName, err)
		return nil, err
	}

	return result, nil
}

// extractUserPullSecrets partitions secretList into user-owned entries and reports whether any
// entry matched a name in managedSecrets (indicating it is managed by Rancher).
// String entries are normalized to LocalObjectReference format ({name: str}) on the way out.
func extractUserPullSecrets(secretList []any, managedSecrets map[string]struct{}) ([]any, bool) {
	var userEntries []any
	hasManagedEntry := false
	for _, entry := range secretList {
		switch v := entry.(type) {
		case string:
			if _, isKnown := managedSecrets[v]; isKnown {
				hasManagedEntry = true
			} else {
				userEntries = append(userEntries, map[string]string{"name": v})
			}
		case map[string]any:
			n, _ := v["name"].(string)
			if _, isKnown := managedSecrets[n]; isKnown {
				hasManagedEntry = true
			} else {
				userEntries = append(userEntries, entry)
			}
		default:
			userEntries = append(userEntries, entry)
		}
	}
	return userEntries, hasManagedEntry
}

func (s *Operations) managePullSecrets(repo *catalog.ClusterRepo, systemDefaultRegistry string, namespace string, releaseName string, existingManagedNames []string) ([]string, error) {
	if systemDefaultRegistry == "" || len(repo.Spec.DefaultImagePullSecrets) == 0 {
		if err := s.deleteStaleHelmOpSecrets([]string{}, existingManagedNames, namespace, releaseName); err != nil {
			return nil, err
		}
		return []string{}, nil
	}

	var secretNames []string
	for _, secret := range repo.Spec.DefaultImagePullSecrets {
		pullSec, err := s.secretCache.Get(namespaces.System, secret.Name)
		if err != nil {
			log.Errorf("[helmop] managePullSecrets: failed to get source secret %q from namespace %q: %v", secret.Name, namespaces.System, err)
			return nil, err
		}

		if pullSec.Labels == nil {
			continue
		}

		// only allow secrets which are currently configured as global pull secrets (source),
		// or have been delivered downstream by Rancher (agent).
		_, isSourceSecret := pullSec.Labels[cluster.SourcePullSecretLabel]
		_, isAgentSecret := pullSec.Labels[cluster.AgentPullSecretLabel]
		if !isSourceSecret && !isAgentSecret {
			continue
		}

		if pullSec.Type != corev1.SecretTypeDockerConfigJson {
			continue
		}

		if len(pullSec.Data[corev1.DockerConfigJsonKey]) == 0 {
			continue
		}

		// Filter the source secret to only the entry matching the configured SDR.
		filteredData, err := cluster.FilterDockerConfigJson(systemDefaultRegistry, pullSec.Data)
		if err != nil {
			if errors.Is(err, cluster.ErrRegistryHostnameNotFound) {
				continue
			}
			log.Errorf("[helmop] managePullSecrets: failed to filter docker config for registry %q from secret %q: %v", systemDefaultRegistry, secret.Name, err)
			return nil, err
		}

		if filteredData == nil {
			log.Tracef("[helmop] managePullSecrets: secret %q has no entry for any configured SDR, skipping", secret.Name)
			continue
		}

		// Scope the copied secret name to this release so that multiple charts in the same
		// namespace don't interfere with each others secrets during uninstallation.
		secretName := name.SafeConcatName(releaseName, secret.Name)

		existingSec, err := s.secretCache.Get(namespace, secretName)
		if err != nil && !apierrors.IsNotFound(err) {
			log.Errorf("[helmop] managePullSecrets: unexpected error looking up existing secret %q in namespace %q: %v", secretName, namespace, err)
			return nil, err
		}

		if existingSec == nil {
			newSec := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
					Labels: map[string]string{
						helmOpManagedPullSecretLabel: "true",
						helmOpReleaseLabelKey:        releaseName,
					},
				},
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: filteredData,
				},
				Type: corev1.SecretTypeDockerConfigJson,
			}
			_, err = s.secrets.Create(newSec)
			if err != nil {
				log.Errorf("[helmop] managePullSecrets: failed to create secret %q in namespace %q: %v", secretName, namespace, err)
				return nil, err
			}
			secretNames = append(secretNames, secretName)
			continue
		}

		// Only manage secrets that we created, skip user-created secrets that happen to share the name.
		if existingSec.Labels != nil && existingSec.Labels[helmOpManagedPullSecretLabel] != "true" {
			log.Debugf("[helmop] managePullSecrets: secret %q in namespace %q is not Rancher-managed, skipping update", secretName, namespace)
			continue
		}

		if existingSec.Data == nil {
			existingSec.Data = make(map[string][]byte)
		}

		if !bytes.Equal(existingSec.Data[corev1.DockerConfigJsonKey], filteredData) {
			existingSec = existingSec.DeepCopy()
			existingSec.Data[corev1.DockerConfigJsonKey] = filteredData
			_, err = s.secrets.Update(existingSec)
			if err != nil {
				log.Errorf("[helmop] managePullSecrets: failed to update secret %q in namespace %q: %v", secretName, namespace, err)
				return nil, err
			}
		}
		secretNames = append(secretNames, secretName)
	}

	if err := s.deleteStaleHelmOpSecrets(secretNames, existingManagedNames, namespace, releaseName); err != nil {
		return nil, err
	}

	log.Debugf("[helmop] managePullSecrets: completed, returning %d secret names: %v", len(secretNames), secretNames)
	return secretNames, nil
}

// deleteStaleHelmOpSecrets deletes managed pull secrets for the given release that are no longer
// active. Any name not in activeSecretNames is deleted.
func (s *Operations) deleteStaleHelmOpSecrets(activeSecretNames []string, existingManagedNames []string, namespace string, releaseName string) error {
	active := make(map[string]struct{}, len(activeSecretNames))
	for _, name := range activeSecretNames {
		active[name] = struct{}{}
	}

	for _, secretName := range existingManagedNames {
		if _, isActive := active[secretName]; isActive {
			continue
		}

		log.Debugf("[helmop] deleteStaleHelmOpSecrets: deleting stale managed secret %q from namespace %q (release %q)", secretName, namespace, releaseName)
		if err := s.secrets.Delete(namespace, secretName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			log.Errorf("[helmop] deleteStaleHelmOpSecrets: failed to delete stale secret %q in namespace %q: %v", secretName, namespace, err)
			return err
		}
	}

	return nil
}

// deleteReleasePullSecrets removes all Rancher-managed pull secrets for the given release from
// the namespace. Called on uninstall to clean up secrets created during install/upgrade.
func (s *Operations) deleteReleasePullSecrets(namespace string, release string) error {
	existing, err := s.listExistingManagedSecretNames(namespace, release)
	if err != nil {
		return err
	}
	return s.deleteStaleHelmOpSecrets([]string{}, existing, namespace, release)
}

func getValueAtPath(data map[string]any, path ...string) (any, bool) {
	current := any(data)
	for i, key := range path {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := cm[key]
		if !ok {
			return nil, false
		}
		if i == len(path)-1 {
			return v, true
		}
		current = v
	}
	return nil, false
}

func setValueAtPath(data map[string]any, value any, path ...string) {
	current := data
	for i := 0; i < len(path)-1; i++ {
		next, ok := current[path[i]].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[path[i]] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}
