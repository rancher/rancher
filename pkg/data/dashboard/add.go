package dashboard

import (
	"context"

	"github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/client-go/kubernetes"
)

func EarlyData(ctx context.Context, k8s kubernetes.Interface) error {
	return addCattleGlobalNamespaces(ctx, k8s)
}

func Add(ctx context.Context, wrangler *wrangler.Context, addLocal, removeLocal, embedded bool) error {
	if !features.MCMAgent.Enabled() {
		if _, err := management.BootstrapAdmin(wrangler); err != nil {
			return err
		}
	}
	if addLocal {
		if err := addLocalCluster(embedded, wrangler); err != nil {
			return err
		}
	} else if removeLocal {
		if err := removeLocalCluster(wrangler); err != nil {
			return err
		}
	}

	if err := addSetting(); err != nil {
		return err
	}

	if err := addRepos(wrangler); err != nil {
		return err
	}

	if err := AddFleetRoles(wrangler); err != nil {
		return err
	}

	return addUnauthenticatedRoles(wrangler.Apply)
}
