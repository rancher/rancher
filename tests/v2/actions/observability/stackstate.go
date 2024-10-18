package observability

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/wait"
	log "github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	StackstateName         = "stackstate"
	ObservabilitySteveType = "configurations.observability.rancher.io"
	CrdGroup               = "observability.rancher.io"
	ApiExtenisonsCRD       = "apiextensions.k8s.io.customresourcedefinition"
)

// NewStackstateConfiguration is a constructor that takes in the configuration and creates an unstructured type to install the CRD
func NewStackstateCRDConfiguration(namespace string, stackstateCRDConfig StackStateConfigs) *unstructured.Unstructured {

	crdConfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      StackstateName,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"url":          stackstateCRDConfig.Url,
				"serviceToken": stackstateCRDConfig.ServiceToken,
			},
		},
	}
	return crdConfig
}


// InstallNodeDriver is a helper that utilizes the rancher client and add the stackstate domains to whitelist them.
func InstallNodeDriver(client *rancher.Client, whitelistDomains []string) error {

	nodedriver := &management.NodeDriver{
		Name:             StackstateName,
		Active:           true,
		WhitelistDomains: whitelistDomains,
	}

	stackstateNodeDriver, err := client.Management.NodeDriver.Create(nodedriver)
	if err != nil {
		return err
	}

	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		resp, err := client.Management.NodeDriver.ByID(stackstateNodeDriver.ID)
		if err != nil {
			return false, err
		}

		if resp.State == "downloading" {
			return true, nil
		}
		return false, nil
	})

	return err
}

// InstallStackstateCRD is a helper that utilizes the rancher client and installs the stackstate crds.
func InstallStackstateCRD(client *rancher.Client) error {

	stackstateCRDConfig := apiextv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{Kind: "CustomResourceDefinition", APIVersion: "apiextensions.k8s.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: ObservabilitySteveType},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: CrdGroup,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				0: {Name: "v1beta1",
					Served:  true,
					Storage: true,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:   "configurations",
				Singular: "configuration",
				Kind:     "Configuration",
				ListKind: "ConfigurationList",
			},
			Scope: "Namespaced",
		},
	}
	crd, err := client.Steve.SteveType(ApiExtenisonsCRD).Create(stackstateCRDConfig)
	if err != nil {
		return err
	}

	client.Session.RegisterCleanupFunc(func() error {
		err := client.Steve.SteveType(ApiExtenisonsCRD).Delete(crd)
		if err != nil {
			return err
		}

		err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
			_, err = client.Steve.SteveType(ApiExtenisonsCRD).ByID(crd.ID)
			if err != nil {
				return false, nil
			}
			return done, nil
		})
		if err != nil {
			return err
		}
		return nil
	})

	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		resp, err := client.Steve.SteveType(ApiExtenisonsCRD).ByID(ObservabilitySteveType)
		if err != nil {
			return false, err
		}

		if resp.ObjectMeta.State.Name == "active" {
			return true, nil
		}
		return false, nil
	})

	return err

}

// AddExtensionsRepo is a helper that utilizes the rancher client and add the ui extensions repo to the list if repositories in the local cluster.
func AddExtensionsRepo(client *rancher.Client, rancherUiPluginsName, uiExtensionGitRepoURL, uiExtensionsRepoBranch string) error {
	log.Info("Adding ui extensions repo to rancher chart repositories in the local cluster.")

	clusterRepoObj := v1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: rancherUiPluginsName,
		},
		Spec: v1.RepoSpec{
			GitRepo:   uiExtensionGitRepoURL,
			GitBranch: uiExtensionsRepoBranch,
		},
	}

	repoObject, err := client.Catalog.ClusterRepos().Create(context.TODO(), &clusterRepoObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	client.Session.RegisterCleanupFunc(func() error {
		err := client.Catalog.ClusterRepos().Delete(context.TODO(), repoObject.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		watchAppInterface, err := client.Catalog.ClusterRepos().Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + repoObject.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting the cluster repo")
			} else if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return err
		}

		return nil
	})

	watchAppInterface, err := client.Catalog.ClusterRepos().Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterRepoObj.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}
	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		repo := event.Object.(*v1.ClusterRepo)

		state := repo.Status.Conditions
		for _, condition := range state {
			if condition.Type == string(v1.RepoDownloaded) && condition.Status == "True" {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}
