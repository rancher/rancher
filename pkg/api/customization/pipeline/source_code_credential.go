package pipeline

import (
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
)

type SourceCodeCredentialHandler struct {
	SourceCodeCredentials      v3.SourceCodeCredentialInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
	SourceCodeRepositories     v3.SourceCodeRepositoryInterface
	SourceCodeRepositoryLister v3.SourceCodeRepositoryLister
}

func SourceCodeCredentialFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "refreshrepos")
	resource.Links["repos"] = apiContext.URLBuilder.Link("repos", resource)
}

func (h SourceCodeCredentialHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {

	if apiContext.Link == "repos" {
		repos, err := h.getReposByCredentialID(apiContext.ID)
		if err != nil {
			return err
		}
		if len(repos) < 1 {
			return h.refreshrepos(apiContext)
		}

		data := []map[string]interface{}{}
		option := &types.QueryOptions{
			Conditions: []*types.QueryCondition{
				types.NewConditionFromString("sourceCodeCredentialId", types.ModifierEQ, []string{apiContext.ID}...),
			},
		}

		if err := access.List(apiContext, apiContext.Version, client.SourceCodeRepositoryType, option, &data); err != nil {
			return err
		}
		apiContext.Type = client.SourceCodeRepositoryType
		apiContext.WriteResponse(http.StatusOK, data)
		return nil
	}

	return httperror.NewAPIError(httperror.NotFound, "Link not found")
}

func (h *SourceCodeCredentialHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {

	switch actionName {
	case "refreshrepos":
		return h.refreshrepos(apiContext)
	}

	return httperror.NewAPIError(httperror.InvalidAction, "unsupported action")
}

func (h *SourceCodeCredentialHandler) refreshrepos(apiContext *types.APIContext) error {

	ns, name := ref.Parse(apiContext.ID)
	credential, err := h.SourceCodeCredentialLister.Get(ns, name)
	if err != nil {
		return err
	}
	if _, err := refreshReposByCredential(h.SourceCodeRepositories, h.SourceCodeRepositoryLister, credential); err != nil {
		return err
	}
	data := []map[string]interface{}{}
	option := &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString("sourceCodeCredentialId", types.ModifierEQ, []string{apiContext.ID}...),
		},
	}

	if err := access.List(apiContext, apiContext.Version, client.SourceCodeRepositoryType, option, &data); err != nil {
		return err
	}
	apiContext.Type = client.SourceCodeRepositoryType
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *SourceCodeCredentialHandler) getReposByCredentialID(sourceCodeCredentialID string) ([]*v3.SourceCodeRepository, error) {
	result := []*v3.SourceCodeRepository{}
	repositories, err := h.SourceCodeRepositoryLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, repo := range repositories {
		if repo.Spec.SourceCodeCredentialName == sourceCodeCredentialID {
			result = append(result, repo)
		}
	}
	return result, nil
}
