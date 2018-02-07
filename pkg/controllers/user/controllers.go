package user

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/user/authz"
	"github.com/rancher/rancher/pkg/controllers/user/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/user/secret"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) error {
	nodesyncer.Register(cluster)
	healthsyncer.Register(ctx, cluster)
	authz.Register(cluster)
	secret.Register(cluster)
	helm.Register(cluster)

	return registerUserOnly(ctx, cluster.UserOnlyContext())
}
