package pipeline

import (
	"encoding/json"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type SourceCodeRepositoryHandler struct {
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
	SourceCodeRepositoryLister v3.SourceCodeRepositoryLister
	ClusterPipelineLister      v3.ClusterPipelineLister
}

func SourceCodeRepositoryFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.Links["pipeline"] = apiContext.URLBuilder.Link("pipeline", resource)
}

func (h SourceCodeRepositoryHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {

	if apiContext.Link == "pipeline" {
		ns, name := ref.Parse(apiContext.ID)
		repo, err := h.SourceCodeRepositoryLister.Get(ns, name)
		if err != nil {
			return err
		}
		ns, name = ref.Parse(repo.Spec.SourceCodeCredentialName)
		cred, err := h.SourceCodeCredentialLister.Get(ns, name)
		if err != nil {
			return err
		}
		clusterPipeline, err := h.ClusterPipelineLister.Get(cred.Spec.ClusterName, cred.Spec.ClusterName)
		if err != nil {
			return err
		}
		remote, err := remote.New(*clusterPipeline, repo.Spec.SourceCodeType)
		if err != nil {
			return err
		}

		m := map[string]interface{}{}

		branch := apiContext.Request.URL.Query().Get("branch")
		if branch == "" {
			branch, err = remote.GetDefaultBranch(repo.Spec.URL, cred.Spec.AccessToken)
			if err != nil {
				return err
			}
		}
		m["branch"] = branch

		content, err := remote.GetPipelineFileInRepo(repo.Spec.URL, branch, cred.Spec.AccessToken)
		if err != nil {
			return err
		}
		if content != nil {
			spec, err := fromYaml(content)
			if err != nil {
				return err
			}
			m["pipeline"] = spec
		}

		bytes, err := json.Marshal(m)
		if err != nil {
			return err
		}
		apiContext.Response.Write(bytes)
		return nil
	}

	return httperror.NewAPIError(httperror.NotFound, "Link not found")
}
