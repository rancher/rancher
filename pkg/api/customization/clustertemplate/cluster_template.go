package clustertemplate

import (
	"net/http"
	"sort"
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

type Wrapper struct {
	ClusterTemplates              v3.ClusterTemplateInterface
	ClusterTemplateLister         v3.ClusterTemplateLister
	ClusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	ClusterTemplateRevisions      v3.ClusterTemplateRevisionInterface
}

func (w Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.Links["revisions"] = apiContext.URLBuilder.Link("revisions", resource)
}

func (w Wrapper) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "revisions":
		var template client.ClusterTemplate
		if err := access.ByID(apiContext, &managementschema.Version, client.ClusterTemplateType, apiContext.ID, &template); err != nil {
			return err
		}
		conditions := []*types.QueryCondition{
			types.NewConditionFromString(client.ClusterTemplateRevisionFieldClusterTemplateID, types.ModifierEQ, []string{template.ID}...),
		}
		var templateVersions []map[string]interface{}
		if err := access.List(apiContext, &managementschema.Version, client.ClusterTemplateRevisionType, &types.QueryOptions{Conditions: conditions}, &templateVersions); err != nil {
			return err
		}
		sort.SliceStable(templateVersions, func(i, j int) bool {
			val1, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(templateVersions[i], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			val2, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(templateVersions[j], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			return val1.After(val2)
		})
		apiContext.Type = client.ClusterTemplateRevisionType
		apiContext.WriteResponse(http.StatusOK, templateVersions)
		return nil
	}
	return nil
}
