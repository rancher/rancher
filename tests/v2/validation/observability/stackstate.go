package observability

import (
	"fmt"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/shepherd/pkg/config"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	rancherUIPlugins           = "rancher-ui-plugins"
	uiExtensionsRepo           = "https://github.com/rancher/ui-plugin-charts"
	uiGitBranch                = "main"
	StackStateCRDConfigFileKey = "stackstateCRDConfigs"
)

var (
	clusterRepoObj = v1.ClusterRepo{
		ObjectMeta: meta.ObjectMeta{
			Name: rancherUIPlugins,
		},
		Spec: v1.RepoSpec{
			GitRepo:   uiExtensionsRepo,
			GitBranch: uiGitBranch,
		},
	}
)

// Add comment
type StackstateCRD struct {
	ServiceToken string `json:"serviceToken" yaml:"serviceToken"`
	Url          string `json:"url" yaml:"url"`
}


// Add comment
func NewStackstateCRDConfig(namespace string) unstructured.Unstructured {
	var stackstateCRDConfigs StackstateCRD

	fmt.Printf("Loading configuration from: %s\n", StackStateCRDConfigFileKey)

	config.LoadConfig(StackStateCRDConfigFileKey, &stackstateCRDConfigs)

	crdConfig := unstructured.Unstructured{}
	crdConfig.Object["url"] = 	stackstateCRDConfigs.Url
	crdConfig.Object["serviceToken"] = stackstateCRDConfigs.ServiceToken
	crdConfig.SetNamespace(namespace)



	return crdConfig
}
