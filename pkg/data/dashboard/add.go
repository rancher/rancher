package dashboard

import (
	"context"

	managementdata "github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/rancher/pkg/wrangler"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

func EarlyData(ctx context.Context, k8s kubernetes.Interface) error {
	return addCattleGlobalNamespaces(ctx, k8s)
}

func Add(ctx context.Context, wrangler *wrangler.Context, addLocal, removeLocal, embedded bool) error {
	adminName, err := managementdata.BootstrapAdmin(wrangler, true)
	if apierror.IsNotFound(err) {
		// ignore if users type doesn't exist
		adminName = ""
	} else if err != nil {
		return err
	}

	if addLocal {
		if err := addLocalCluster(embedded, adminName, wrangler); err != nil {
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

	if err := addRepos(ctx, wrangler); err != nil {
		return err
	}

	return nil
}
