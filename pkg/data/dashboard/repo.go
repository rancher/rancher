package dashboard

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/rancher/rancher/pkg/settings"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	packagedRepos = map[string]string{
		"rancher-charts":         settings.ChartDefaultBranch.Get(),
		"rancher-partner-charts": settings.PartnerChartDefaultBranch.Get(),
	}
	prefix = "rancher-"
)

func addRepo(wrangler *wrangler.Context, repoName, branchName string) error {
	repo, err := wrangler.Catalog.ClusterRepo().Get(repoName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = wrangler.Catalog.ClusterRepo().Create(&v1.ClusterRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name: repoName,
			},
			Spec: v1.RepoSpec{
				GitRepo:   "https://git.rancher.io/" + strings.TrimPrefix(repoName, prefix),
				GitBranch: branchName,
			},
		})
	} else if err == nil && repo.Spec.GitBranch != packagedRepos[repoName] {
		repo.Spec.GitBranch = packagedRepos[repoName]
		_, err = wrangler.Catalog.ClusterRepo().Update(repo)
	}

	return err
}

func addRepos(ctx context.Context, wrangler *wrangler.Context) error {
	for repoName, branchName := range packagedRepos {
		if err := addRepo(wrangler, repoName, branchName); err != nil {
			return err
		}
	}

	return nil
}
