package nodetemplate

import (
	"strings"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

type Formatter struct {
	NodePoolLister v3.NodePoolLister
	NodeLister     v3.NodeLister
	UserLister     v3.UserLister
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
			break
		}
	}
	// Only check node references if remove link is still present
	if _, ok := resource.Links["remove"]; ok {
		nodes, err := ntf.NodeLister.List("", labels.Everything())
		if err != nil {
			logrus.Warnf("Failed to determine if Node Template is being used. Error: %v", err)
			return
		}
		for _, node := range nodes {
			if node.Spec.NodeTemplateName == resource.ID {
				delete(resource.Links, "remove")
				break
			}
		}
	}

	creatorID, _ := resource.Values["creatorId"].(string)

	user, err := ntf.UserLister.Get("", creatorID)
	if err != nil {
		if !errors.IsNotFound(err) {
			logrus.Warnf("[NodeTemplate Formatter] Failed to to get user associated with creatorId [%s]. Error: %v", creatorID, err)
		}
		return
	}

	for _, principalID := range user.PrincipalIDs {
		if strings.HasPrefix(principalID, "local://") {
			resource.Values["principalId"] = principalID
			return
		}
	}
}
