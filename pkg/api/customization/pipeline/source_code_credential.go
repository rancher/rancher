package pipeline

import (
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/pipeline/providers"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/client/project/v3"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
)

const (
	actionRefreshRepos = "refreshrepos"
	actionLogout       = "logout"
	linkRepos          = "repos"
)

type SourceCodeCredentialHandler struct {
	SourceCodeCredentials      v3.SourceCodeCredentialInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
	SourceCodeRepositories     v3.SourceCodeRepositoryInterface
	SourceCodeRepositoryLister v3.SourceCodeRepositoryLister
}

func SourceCodeCredentialFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, actionRefreshRepos)
	resource.AddAction(apiContext, actionLogout)
	resource.Links[linkRepos] = apiContext.URLBuilder.Link(linkRepos, resource)
}

func (h SourceCodeCredentialHandler) ListHandler(request *types.APIContext, next types.RequestHandler) error {
	request.Query.Set("logout_ne", "true")
	return handler.ListHandler(request, next)
}

func (h SourceCodeCredentialHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link == linkRepos {
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
	case actionRefreshRepos:
		return h.refreshrepos(apiContext)
	case actionLogout:
		return h.logout(apiContext)
	}

	return httperror.NewAPIError(httperror.InvalidAction, "unsupported action")
}

func (h *SourceCodeCredentialHandler) refreshrepos(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	credential, err := h.SourceCodeCredentialLister.Get(ns, name)
	if err != nil {
		return err
	}

	_, projID := ref.Parse(credential.Spec.ProjectName)
	scpConfig, err := providers.GetSourceCodeProviderConfig(credential.Spec.SourceCodeType, projID)
	if err != nil {
		return err
	}

	if _, err := providers.RefreshReposByCredential(h.SourceCodeRepositories, h.SourceCodeRepositoryLister, credential, scpConfig); err != nil {
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

func (h *SourceCodeCredentialHandler) logout(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	credential, err := h.SourceCodeCredentialLister.Get(ns, name)
	if err != nil {
		return err
	}
	credential.Status.Logout = true
	if _, err := h.SourceCodeCredentials.Update(credential); err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

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
