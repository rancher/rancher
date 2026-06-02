package helmop

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/cluster"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

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

	supportsPullSecrets := false
	for i := range cmds {
		var baseValues, values map[string]any
		if err := yaml.Unmarshal(cmds[i].ChartBaseValues, &baseValues); err != nil {
			log.Errorf("[helmop] createNamespaceAndPullSecrets: failed to unmarshal chart base values: %v", err)
			return err
		}

		if err := yaml.Unmarshal(cmds[i].Values, &values); err != nil {
			log.Errorf("[helmop] createNamespaceAndPullSecrets: failed to unmarshal chart base values: %v", err)
			return err
		}

		sdrConfigured, err := s.SystemDefaultRegistryConfigured(values)
		if err != nil {
			return err
		}

		if !sdrConfigured {
			continue
		}

		supportsPullSecrets, err = s.chartSupportsImagePullSecrets(baseValues)
		if err != nil {
			return err
		}

		if supportsPullSecrets {
			break
		}
	}

	if !supportsPullSecrets {
		return nil
	}

	pullSecrets, err := s.createOrUpdatePullSecrets(ns.Name, clusterRepoName)
	if err != nil {
		return err
	}

	log.Tracef("[helmop] createOrUpdatePullSecrets returned %d secrets for namespace=%q repo=%q: %v", len(pullSecrets), ns.Name, clusterRepoName, pullSecrets)
	for i := range cmds {
		log.Tracef("[helmop] injecting pull secrets into command %d (release=%q, namespace=%q, valuesLen=%d, chartBaseValuesLen=%d)", i, cmds[i].ReleaseName, cmds[i].ReleaseNamespace, len(cmds[i].Values), len(cmds[i].ChartBaseValues))
		var baseValues, values map[string]any
		if err := yaml.Unmarshal(cmds[i].ChartBaseValues, &baseValues); err != nil {
			log.Errorf("[helmop] injectPullSecrets: failed to unmarshal chart base values: %v", err)
			return err
		}
		if err := yaml.Unmarshal(cmds[i].Values, &values); err != nil {
			log.Errorf("[helmop] injectPullSecrets: failed to unmarshal chart base values: %v", err)
			return err
		}
		cmds[i].Values, err = s.injectPullSecrets(cmds[i].ReleaseName, baseValues, values, pullSecrets)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Operations) SystemDefaultRegistryConfigured(values map[string]any) (bool, error) {
	v, defined := getValueAtPath(values, "global", "cattle", "systemDefaultRegistry")
	return defined && v != "", nil
}

func (s *Operations) chartSupportsImagePullSecrets(baseValues map[string]any) (bool, error) {
	for _, path := range imagePullSecretPaths {
		pathStr := strings.Join(path, ".")
		if _, declared := getValueAtPath(baseValues, path...); !declared {
			continue
		}
		log.Tracef("[helmop] injectPullSecrets: path %q is declared in chart base values", pathStr)
		return true, nil
	}
	return false, nil
}

func (s *Operations) injectPullSecrets(releaseName string, baseValues map[string]any, confValues map[string]any, secretNames []string) ([]byte, error) {
	pullSecrets := make([]map[string]string, len(secretNames))
	for i, e := range secretNames {
		pullSecrets[i] = map[string]string{"name": e}
	}

	for _, path := range imagePullSecretPaths {
		pathStr := strings.Join(path, ".")
		if _, declared := getValueAtPath(baseValues, path...); !declared {
			continue
		}
		if value, exists := getValueAtPath(confValues, path...); exists {
			if secrets, ok := value.([]any); ok && len(secrets) > 0 {
				continue
			}
		}
		log.Tracef("[helmop] injectPullSecrets: injecting %d pull secrets at path %q for release %s: %v", len(pullSecrets), pathStr, releaseName, pullSecrets)
		setValueAtPath(confValues, pullSecrets, path...)
	}

	result, err := json.Marshal(confValues)
	if err != nil {
		log.Errorf("[helmop] injectPullSecrets: failed to marshal modified values: %v", err)
		return nil, err
	}

	return result, nil
}

func (s *Operations) createOrUpdatePullSecrets(namespace string, repoName string) ([]string, error) {
	repo, err := s.clusterReposCache.Get(repoName)
	if err != nil {
		log.Errorf("[helmop] createOrUpdatePullSecrets: failed to get cluster repo %q from cache: %v", repoName, err)
		return nil, err
	}

	if len(repo.Spec.DefaultImagePullSecrets) == 0 {
		return nil, nil
	}

	var secretNames []string
	for _, secret := range repo.Spec.DefaultImagePullSecrets {
		pullSec, err := s.secretCache.Get(namespaces.System, secret.Name)
		if err != nil {
			log.Errorf("[helmop] createOrUpdatePullSecrets: failed to get source secret %q from namespace %q: %v", secret.Name, namespaces.System, err)
			return nil, err
		}

		if pullSec.Labels == nil {
			continue
		}

		_, isCopiedSecret := pullSec.Labels[cluster.CopiedPullSecretLabel]
		_, isSourceSecret := pullSec.Labels[cluster.SourcePullSecretLabel]
		_, isAgentSecret := pullSec.Labels["management.cattle.io/cattle-cluster-agent-pull-secret"]
		if !isCopiedSecret && !isSourceSecret && !isAgentSecret {
			continue
		}

		existingSec, err := s.secretCache.Get(namespace, secret.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				log.Errorf("[helmop] createOrUpdatePullSecrets: unexpected error looking up existing secret %q in namespace %q: %v", secret.Name, namespace, err)
				return nil, err
			}
		}

		if existingSec == nil {
			newSec := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret.Name,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: pullSec.Data[corev1.DockerConfigJsonKey],
				},
				Type: corev1.SecretTypeDockerConfigJson,
			}
			_, err = s.secrets.Create(newSec)
			if err != nil {
				log.Errorf("[helmop] createOrUpdatePullSecrets: failed to create secret %q in namespace %q: %v", secret.Name, namespace, err)
				return nil, err
			}
			secretNames = append(secretNames, secret.Name)
			continue
		}

		existingSec, needsUpdate, err := secretNeedsUpdate(existingSec, pullSec)
		if needsUpdate {
			_, err = s.secrets.Update(existingSec)
			if err != nil {
				log.Errorf("[helmop] createOrUpdatePullSecrets: failed to update secret %q in namespace %q: %v", secret.Name, namespace, err)
				return nil, err
			}
		}

		secretNames = append(secretNames, secret.Name)
	}

	log.Debugf("[helmop] createOrUpdatePullSecrets: completed, returning %d secret names: %v", len(secretNames), secretNames)
	return secretNames, nil
}

func secretNeedsUpdate(existing, current *corev1.Secret) (*corev1.Secret, bool, error) {
	dataOutdated := existing.Data[corev1.DockerConfigJsonKey] == nil || !bytes.Equal(existing.Data[corev1.DockerConfigJsonKey], current.Data[corev1.DockerConfigJsonKey])

	if !dataOutdated {
		return existing, false, nil
	}

	existing = existing.DeepCopy()
	existing.Data[corev1.DockerConfigJsonKey] = current.Data[corev1.DockerConfigJsonKey]

	return existing, true, nil
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
