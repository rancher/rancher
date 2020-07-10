package nodepool

import (
	"strings"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

type Formatter struct {
	NodeTemplateLister v3.NodeTemplateLister
}

func (ntf *Formatter) Formatter(request *types.APIContext, resource *types.RawResource) {
	nodeTemplateID, _ := resource.Values[client.NodePoolFieldNodeTemplateID].(string)
	if nodeTemplateID == "" {
		return
	}

	// id is namespace:name
	parts := strings.Split(nodeTemplateID, ":")
	if len(parts) != 2 {
		return
	}

	template, err := ntf.NodeTemplateLister.Get(parts[0], parts[1])
	if err != nil {
		logrus.Warnf("Failed to get nodeTemplate driver for nodePool %v. Error: %v", resource.ID, err)
		return
	}

	resource.Values["driver"] = template.Spec.Driver
}
