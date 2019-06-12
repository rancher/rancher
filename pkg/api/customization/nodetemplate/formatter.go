package nodetemplate

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

type Formatter struct {
	NodePoolLister v3.NodePoolLister
}

func (ntf *Formatter) Formatter(request *types.APIContext, resource *types.RawResource) {
	if !filterToOwnNamespace(request, resource) {
		return
	}

	pools, err := ntf.NodePoolLister.List("", labels.Everything())
	if err != nil {
		logrus.Warnf("Failed to determine if Node Template is being used. Error: %v", err)
		return
	}

	for _, pool := range pools {
		if pool.Spec.NodeTemplateName == resource.ID {
			delete(resource.Links, "remove")
			return
		}
	}
}

// TODO: This should go away, it is simply a hack to get the watch on nodetemplates to filter appropriately until that system is refactored
func filterToOwnNamespace(request *types.APIContext, resource *types.RawResource) bool {
	user := request.Request.Header.Get("Impersonate-User")
	if user == "" {
		logrus.Errorf(
			"%v",
			httperror.NewAPIError(httperror.ServerError, "There was an error authorizing the user"))
		return false
	}

	if !strings.HasPrefix(resource.ID, user+":") {
		resource.ID = ""
		resource.Values = map[string]interface{}{}
		resource.Links = map[string]string{}
		resource.Actions = map[string]string{}
		resource.Type = ""

		return false
	}

	return true
}
