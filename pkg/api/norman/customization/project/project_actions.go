package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	"github.com/rancher/rancher/pkg/generated/compose"
	provisioningcontrollerv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/user"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "exportYaml")
}

type Handler struct {
	Projects                 v3.ProjectInterface
	ProjectLister            v3.ProjectLister
	ClusterManager           *clustermanager.Manager
	ClusterLister            v3.ClusterLister
	ProvisioningClusterCache provisioningcontrollerv1.ClusterCache
	UserMgr                  user.Manager
}

func (h *Handler) Actions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "exportYaml":
		return h.ExportYamlHandler(actionName, action, apiContext)
	}

	return errors.Errorf("unrecognized action %v", actionName)
}

func (h *Handler) ExportYamlHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	namespace, id := ref.Parse(apiContext.ID)
	project, err := h.ProjectLister.Get(namespace, id)
	if err != nil {
		return err
	}
	topkey := compose.Config{}
	topkey.Version = "v3"
	p := client.Project{}
	if err := convert.ToObj(project.Spec, &p); err != nil {
		return err
	}
	topkey.Projects = map[string]client.Project{}
	topkey.Projects[project.Spec.DisplayName] = p
	m, err := convert.EncodeToMap(topkey)
	if err != nil {
		return err
	}
	delete(m["projects"].(map[string]interface{})[project.Spec.DisplayName].(map[string]interface{}), "actions")
	delete(m["projects"].(map[string]interface{})[project.Spec.DisplayName].(map[string]interface{}), "links")
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	buf, err := yaml.JSONToYAML(data)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(buf)
	apiContext.Response.Header().Set("Content-Type", "text/yaml")
	http.ServeContent(apiContext.Response, apiContext.Request, "exportYaml", time.Now(), reader)
	return nil
}

func getID(id interface{}) (string, error) {
	s, ok := id.(string)
	if !ok {
		return "", fmt.Errorf("could not convert %v", id)
	}

	split := strings.Split(s, ":")
	return split[0] + ":" + split[len(split)-1], nil
}

// isProvisionedRke2Cluster check to see if this is a rancher provisioned rke2 cluster
func isProvisionedRke2Cluster(cluster *v3.Cluster) bool {
	return cluster.Status.Provider == v32.ClusterDriverRke2 && imported.IsAdministratedByProvisioningCluster(cluster)
}

// parseKubeApiServerArgs parses the "kube-apiserver-arg" available in the
// clusters' MachineGlobalConfig to a map. The arguments are expected to
// follow the "key=value" format. Arguments that don't follow this format
// are ignored.
func parseKubeAPIServerArgs(provisioningCluster *provisioningv1.Cluster) map[string]string {
	result := make(map[string]string)

	rawArgs, ok := provisioningCluster.Spec.RKEConfig.MachineGlobalConfig.Data["kube-apiserver-arg"]
	if !ok || rawArgs == nil {
		return result
	}

	args, ok := rawArgs.([]any)
	if !ok || args == nil {
		return result
	}

	for _, arg := range args {
		s, ok := arg.(string)
		if !ok {
			continue
		}
		key, value, found := strings.Cut(s, "=")
		if found {
			result[key] = value
		}
	}
	return result
}
