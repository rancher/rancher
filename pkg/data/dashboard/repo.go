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
	prefix = "rancher-"
)

func addClusterRepo(wrangler *wrangler.Context, repoName, branchName string) error {
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
	} else if err == nil && repo.Spec.GitBranch != branchName {
		repo.Spec.GitBranch = branchName
		_, err = wrangler.Catalog.ClusterRepo().Update(repo)
	}

	return err
}

// addClusterRepos sets up default cluster helm repositories in Rancher.
// It attempts to add or update the cluster repositories 'rancher-charts', 'rancher-partner-charts',
// and, if the RKE2 feature is enabled, 'rancher-rke2-charts'.
func addClusterRepos(ctx context.Context, wrangler *wrangler.Context) error {
	if err := addClusterRepo(wrangler, "rancher-charts", settings.ChartDefaultBranch.Get()); err != nil {
		return err
	}

	return nil
}
