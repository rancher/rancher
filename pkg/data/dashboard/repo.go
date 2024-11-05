package dashboard

import (
	"strings"

	"github.com/rancher/rancher/pkg/features"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/rancher/rancher/pkg/settings"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultURL = "https://git.rancher.io/"

var (
	prefix = "rancher-"
)

func addRepo(wrangler *wrangler.Context, repoName, repoURL, branchName string) error {
	if repoURL == "" || repoURL == defaultURL {
		repoURL = defaultURL + strings.TrimPrefix(repoName, prefix)
	} else {
		logrus.Warnf(
			"Charts URL for %q set to %q, which is not the default (%q). "+
				"If Rancher has issues finding charts, consider resetting this to the default value",
			repoName,
			repoURL,
			defaultURL,
		)
	}

	repo, err := wrangler.Catalog.ClusterRepo().Get(repoName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = wrangler.Catalog.ClusterRepo().Create(&v1.ClusterRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name: repoName,
			},
			Spec: v1.RepoSpec{
				GitRepo:   repoURL,
				GitBranch: branchName,
			},
		})
	} else if err == nil && (repo.Spec.GitRepo != repoURL || repo.Spec.GitBranch != branchName) {
		repo.Spec.GitRepo = repoURL
		repo.Spec.GitBranch = branchName
		_, err = wrangler.Catalog.ClusterRepo().Update(repo)
	}

	return err
}

// addRepos upserts the rancher-charts, rancher-partner-charts and rancher-rke2-charts ClusterRepos
func addRepos(wrangler *wrangler.Context) error {
	if err := addRepo(
		wrangler,
		"rancher-charts",
		settings.ChartDefaultURL.Get(),
		settings.ChartDefaultBranch.Get(),
	); err != nil {
		return err
	}

	if err := addRepo(
		wrangler,
		"rancher-partner-charts",
		settings.PartnerChartDefaultURL.Get(),
		settings.PartnerChartDefaultBranch.Get(),
	); err != nil {
		return err
	}

	if features.RKE2.Enabled() {
		if err := addRepo(
			wrangler,
			"rancher-rke2-charts",
			settings.RKE2ChartDefaultURL.Get(),
			settings.RKE2ChartDefaultBranch.Get(),
		); err != nil {
			return err
		}
	}

	return nil
}
