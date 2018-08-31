package nodetemplate

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

type Formatter struct {
	NodePoolLister v3.NodePoolLister
}

func (ntf *Formatter) Formatter(request *types.APIContext, resource *types.RawResource) {
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
