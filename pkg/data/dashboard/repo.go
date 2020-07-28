package dashboard

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addRepos(ctx context.Context, wrangler *wrangler.Context) error {
	// TODO Create ClusterRepo, don't ignore, only do this once, so save some state that this was done
	_, _ = wrangler.Catalog.Repo().Create(&v1.Repo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev-charts",
			Namespace: "default",
		},
		Spec: v1.RepoSpec{
			URL: "https://dev-charts.rancher.io",
		},
	})
	return nil
}
