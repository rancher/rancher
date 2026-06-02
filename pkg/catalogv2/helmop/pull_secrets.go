package helmop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/cluster"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/name"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

// createNamespaceAndPullSecrets is responsible for creating the release namespace for chart on install/upgrade operations and managing the default image pull
// secrets to be used by that chart. It checks if the chart supports image pull secrets and if it does, and that is has a system-default-registry configure. If so,
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

		sdr, sdrDefined := s.SystemDefaultRegistryConfigured(values)

		pullSecrets, err := s.managePullSecrets(sdr, ns.Name, clusterRepoName, cmds[i].ReleaseName)
		if err != nil {
			return err
		}

		if !sdrDefined {
			continue
		}

		log.Tracef("[helmop] injecting pull secrets into command %d (release=%q, namespace=%q, valuesLen=%d, chartBaseValuesLen=%d)", i, cmds[i].ReleaseName, cmds[i].ReleaseNamespace, len(cmds[i].Values), len(cmds[i].ChartBaseValues))
		cmds[i].Values, err = s.injectPullSecrets(cmds[i].ReleaseName, baseValues, values, pullSecrets)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Operations) SystemDefaultRegistryConfigured(values map[string]any) (string, bool) {
	v, defined := getValueAtPath(values, "global", "cattle", "systemDefaultRegistry")
	sdr, ok := v.(string)
	return sdr, defined && ok
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

func (s *Operations) injectPullSecrets(releaseName string, baseValues map[string]any, confValues map[string]any, secretNames []string) ([]byte, error) {
	pullSecrets := make([]map[string]string, len(secretNames))
	for i, e := range secretNames {
		pullSecrets[i] = map[string]string{"name": e}
	}

	for _, path := range imagePullSecretPaths {
		if _, declared := getValueAtPath(baseValues, path...); !declared {
			continue
		}
		if value, exists := getValueAtPath(confValues, path...); exists {
			if secrets, ok := value.([]any); ok && len(secrets) > 0 {
				continue
			}
		}
		log.Tracef("[helmop] injectPullSecrets: injecting %d pull secrets at path %q for release %q: %v", len(pullSecrets), strings.Join(path, "."), releaseName, pullSecrets)
		setValueAtPath(confValues, pullSecrets, path...)
	}

	result, err := json.Marshal(confValues)
	if err != nil {
		log.Errorf("[helmop] injectPullSecrets: failed to marshal modified values for release %q: %v", releaseName, err)
		return nil, err
	}

	return result, nil
}

func (s *Operations) managePullSecrets(systemDefaultRegistry string, namespace string, repoName string, releaseName string) ([]string, error) {
	repo, err := s.clusterReposCache.Get(repoName)
	if err != nil {
		log.Errorf("[helmop] managePullSecrets: failed to get cluster repo %q from cache: %v", repoName, err)
		return nil, err
	}

	if systemDefaultRegistry == "" {
		if err := s.deleteStaleHelmOpSecrets([]string{}, namespace, releaseName); err != nil {
			return nil, err
		}
		return []string{}, nil
	}

	if len(repo.Spec.DefaultImagePullSecrets) == 0 {
		if err := s.deleteStaleHelmOpSecrets([]string{}, namespace, releaseName); err != nil {
			return nil, err
		}
		return nil, nil
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
			if err.Error() == fmt.Sprintf(cluster.ErrRegistryHostnameNotFound, systemDefaultRegistry) {
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
		if err != nil && !errors.IsNotFound(err) {
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
		if existingSec.Labels[helmOpManagedPullSecretLabel] != "true" {
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

	if err := s.deleteStaleHelmOpSecrets(secretNames, namespace, releaseName); err != nil {
		return nil, err
	}

	log.Debugf("[helmop] managePullSecrets: completed, returning %d secret names: %v", len(secretNames), secretNames)
	return secretNames, nil
}

// deleteStaleHelmOpSecrets deletes secrets in the release namespace that were previously created
// by managePullSecrets (identified by helmOpManagedPullSecretLabel and helmOpReleaseLabelKey)
// but are no longer needed. It lists all managed secrets for the given release via label selector
// and deletes any not present in activeSecretNames.
func (s *Operations) deleteStaleHelmOpSecrets(activeSecretNames []string, namespace string, releaseName string) error {
	active := make(map[string]struct{}, len(activeSecretNames))
	for _, name := range activeSecretNames {
		active[name] = struct{}{}
	}

	managed, err := s.secretCache.List(namespace, labels.SelectorFromSet(labels.Set{
		helmOpManagedPullSecretLabel: "true",
		helmOpReleaseLabelKey:        releaseName,
	}))
	if err != nil {
		log.Errorf("[helmop] deleteStaleHelmOpSecrets: failed to list managed secrets in namespace %q for release %q: %v", namespace, releaseName, err)
		return err
	}

	for _, secret := range managed {
		if _, isActive := active[secret.Name]; isActive {
			continue
		}

		log.Debugf("[helmop] deleteStaleHelmOpSecrets: deleting stale managed secret %q from namespace %q (release %q)", secret.Name, namespace, releaseName)
		if err := s.secrets.Delete(namespace, secret.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			log.Errorf("[helmop] deleteStaleHelmOpSecrets: failed to delete stale secret %q in namespace %q: %v", secret.Name, namespace, err)
			return err
		}
	}

	return nil
}

// deleteReleasePullSecrets removes all Rancher-managed pull secrets for each release in cmds
// from the given namespace. This is called on uninstall to clean up secrets created during install/upgrade.
func (s *Operations) deleteReleasePullSecrets(namespace string, release string) error {
	if err := s.deleteStaleHelmOpSecrets([]string{}, namespace, release); err != nil {
		return err
	}
	return nil
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
