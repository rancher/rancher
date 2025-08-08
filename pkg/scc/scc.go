package scc

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/scc/controllers"
	"github.com/rancher/rancher/pkg/scc/deployer"
	"github.com/rancher/rancher/pkg/scc/deployer/params"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/rancher/pkg/wrangler"
)

// StartDeployer sets up the SCCDeployer and registers the related scoped controllers
func StartDeployer(ctx context.Context, wContext *wrangler.Context) error {
	operatorLogger := log.NewLog()
	operatorLogger.Debug("Preparing to deploy scc-operator")

	sccDeployer, err := deployer.NewSCCDeployer(wContext, operatorLogger.WithField("component", "scc-deployer"))
	if err != nil {
		return fmt.Errorf("error creating scc deployer: %v", err)
	}

	initialParams, err := params.ExtractSccOperatorParams()
	if err != nil {
		operatorLogger.Errorf("Failed to extract SCC operator params: %v", err)
		return err
	}
	operatorLogger.Debugf("SCC operator params: %v", initialParams)

	if err = sccDeployer.EnsureDependenciesConfigured(ctx, initialParams); err != nil {
		return fmt.Errorf("cannot start scc-operator deployer, failed to ensure dependencies: %w", err)
	}

	controllers.RegisterDeployer(
		ctx,
		operatorLogger,
		sccDeployer,
		wContext.Apps.Deployment(),
		wContext.Mgmt.Setting(),
	)

	// Handle initial deployment or initial update on leader start
	// TODO: maybe this should be a go routine
	return sccDeployer.EnsureSCCOperatorDeployed(ctx, initialParams)
}
