package yaml

import (
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
)

func NewLinkHandler(proxy http.Handler, manager *clustermanager.Manager, next types.RequestHandler) types.RequestHandler {
	lh := &yamlLinkHandler{
		Proxy:          proxy,
		ClusterManager: manager,
		next:           next,
	}

	return lh.LinkHandler
}

type yamlLinkHandler struct {
	Proxy          http.Handler
	ClusterManager *clustermanager.Manager
	next           types.RequestHandler
}

func (s *yamlLinkHandler) callNext(apiContext *types.APIContext, next types.RequestHandler) error {
	if s.next != nil {
		return s.next(apiContext, next)
	} else if next != nil {
		return next(apiContext, nil)
	}

	return httperror.NewAPIError(httperror.NotFound, "link not found")
}

func (s *yamlLinkHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link != "yaml" {
		return s.callNext(apiContext, next)
	}

	clusterName := s.ClusterManager.ClusterName(apiContext)
	if clusterName == "" {
		return httperror.NewAPIError(httperror.NotFound, "cluster not found")
	}

	schema := apiContext.Schemas.Schema(apiContext.Version, apiContext.Type)
	if schema == nil {
		return fmt.Errorf("failed to find schema " + apiContext.Type)
	}

	data, err := schema.Store.ByID(apiContext, schema, apiContext.ID)
	if err != nil {
		return err
	}

	link, _ := data[".selfLink"].(string)

	if link == "" {
		return httperror.NewAPIError(httperror.NotFound, "self link not found")
	}

	apiContext.Request.URL.Path = fmt.Sprintf("/k8s/clusters/%s%s", clusterName, link)
	s.Proxy.ServeHTTP(apiContext.Response, apiContext.Request)

	return nil
}
