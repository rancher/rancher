package dashboardapi

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/rancher/rancher/pkg/features"
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
	} else if err == nil {
		if repo.Spec.GitBranch != branchName {
			repo.Spec.GitBranch = branchName
			repo, err = wrangler.Catalog.ClusterRepo().Update(repo)
			if err != nil {
				return err
			}
		}

		if repo.Status.Commit != "" {
			repo.Status.Commit = ""
			_, err = wrangler.Catalog.ClusterRepo().UpdateStatus(repo)
		}
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
	if err := addClusterRepo(wrangler, "rancher-partner-charts", settings.PartnerChartDefaultBranch.Get()); err != nil {
		return err
	}

	if features.RKE2.Enabled() {
		if err := addClusterRepo(wrangler, "rancher-rke2-charts", settings.RKE2ChartDefaultBranch.Get()); err != nil {
			return err
		}
	}

	return nil
}
