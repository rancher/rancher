package scc

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/controllers"
	"github.com/rancher/rancher/pkg/scc/deployer"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/rancher/pkg/wrangler"
)

// Start sets up the SCCDeployer and registers the related scoped controllers
func Start(ctx context.Context, wContext *wrangler.Context) error {
	operatorLogger := log.NewLog()
	operatorLogger.Debug("Preparing to deploy scc-operator")

	sccDeployer, err := deployer.NewSCCDeployer(wContext, operatorLogger.WithField("component", "scc-deployer"))
	if err != nil {
		return fmt.Errorf("error creating scc deployer: %v", err)
	}

	if err = sccDeployer.EnsureDependenciesConfigured(ctx); err != nil {
		return fmt.Errorf("cannot start scc-operator deployer, failed to ensure dependencies: %w", err)
	}

	controllers.RegisterDeployers(
		ctx,
		operatorLogger,
		*sccDeployer,
		wContext.Apps.Deployment(),
		wContext.Apps.Deployment().Cache(),
		wContext.Mgmt.Setting(),
	)

	// Initial deployment
	return sccDeployer.EnsureSCCOperatorDeployment(ctx)
}
