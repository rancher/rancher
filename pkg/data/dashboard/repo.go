package dashboard

import (
	"context"
	"strings"

	"github.com/rancher/rancher/pkg/features"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/rancher/rancher/pkg/settings"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	prefix = "rancher-"
)

func addRepo(wrangler *wrangler.Context, repoName, branchName string) error {
	if repoName == "rancher-charts" {
		_, err := wrangler.Catalog.ClusterRepo().Get(repoName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = wrangler.Catalog.ClusterRepo().Create(&v1.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name: repoName,
				},
				Spec: v1.RepoSpec{
					GitRepo:   "https://github.com/chiukapoor/charts/",
					GitBranch: "rancher-v1.28",
				},
			})
		}
		return err
	}
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

// addRepos upserts the rancher-charts, rancher-partner-charts and rancher-rke2-charts ClusterRepos
func addRepos(ctx context.Context, wrangler *wrangler.Context) error {
	if err := addRepo(wrangler, "rancher-charts", settings.ChartDefaultBranch.Get()); err != nil {
		return err
	}
	if err := addRepo(wrangler, "rancher-partner-charts", settings.PartnerChartDefaultBranch.Get()); err != nil {
		return err
	}

	if features.RKE2.Enabled() {
		if err := addRepo(wrangler, "rancher-rke2-charts", settings.RKE2ChartDefaultBranch.Get()); err != nil {
			return err
		}
	}

	return nil
}
