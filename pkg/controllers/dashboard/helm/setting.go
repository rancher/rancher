package helm

import (
	"context"
	"reflect"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/cluster"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func RegisterRepoSettings(ctx context.Context,
	settingController v3.SettingController,
	clusterRepos catalogcontrollers.ClusterRepoController) {

	h := &repoHandler{
		settings:     settingController,
		clusterRepos: clusterRepos,
	}

	settingController.OnChange(ctx, "synchronize-rancher-repo-pull-secrets", h.onRegistryPullSecretsSettingsChange)
}

func (r *repoHandler) onRegistryPullSecretsSettingsChange(_ string, setting *apimgmtv3.Setting) (*apimgmtv3.Setting, error) {
	if setting == nil || (setting.Name != settings.SystemDefaultRegistryPullSecrets.Name && setting.Name != settings.SystemDefaultRegistry.Name) {
		return setting, nil
	}

	rancherRepo, err := r.clusterRepos.Get("rancher-charts", v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	registry, _ := cluster.GetPrivateRegistry(nil)
	if registry == nil || len(registry.PullSecrets) == 0 {
		if len(rancherRepo.Spec.DefaultImagePullSecrets) > 0 {
			rancherRepo = rancherRepo.DeepCopy()
			rancherRepo.Spec.DefaultImagePullSecrets = nil
			_, err = r.clusterRepos.Update(rancherRepo)
			if err != nil {
				return setting, err
			}
		}
		return setting, nil
	}

	var secretReferences []catalogv1.SecretReference
	for _, ps := range registry.PullSecrets {
		secretReferences = append(secretReferences, catalogv1.SecretReference{
			Name:      ps.Name,
			Namespace: namespaces.System,
		})
	}

	// Only update the repo if the pull secrets have actually changed.
	if !reflect.DeepEqual(rancherRepo.Spec.DefaultImagePullSecrets, secretReferences) {
		rancherRepo = rancherRepo.DeepCopy()
		rancherRepo.Spec.DefaultImagePullSecrets = secretReferences
		_, err = r.clusterRepos.Update(rancherRepo)
		if err != nil {
			return setting, err
		}
	}

	return setting, nil
}
