package deployer

import (
	"context"

	"github.com/rancher/rancher/pkg/scc/deployer/clusterrolebinding"
	"github.com/rancher/rancher/pkg/scc/deployer/deployment"
	"github.com/rancher/rancher/pkg/scc/deployer/namespace"
	"github.com/rancher/rancher/pkg/scc/deployer/params"
	"github.com/rancher/rancher/pkg/scc/deployer/serviceaccount"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/rancher/pkg/wrangler"
	appsv1 "k8s.io/api/apps/v1"
)

// SCCDeployer orchestrates the deployment of SCC operator resources
type SCCDeployer struct {
	log log.StructuredLogger

	currentSCCOperatorPodHash string

	// Individual resource deployers
	namespaceDeployer          *namespace.Deployer
	serviceAccountDeployer     *serviceaccount.Deployer
	clusterRoleBindingDeployer *clusterrolebinding.Deployer
	deploymentDeployer         *deployment.Deployer
}

// NewSCCDeployer creates a new SCCDeployer with initialized resource deployers
func NewSCCDeployer(wContext *wrangler.Context, log log.StructuredLogger) (*SCCDeployer, error) {
	return &SCCDeployer{
		log:                        log,
		namespaceDeployer:          namespace.NewDeployer(log, wContext.Core.Namespace()),
		serviceAccountDeployer:     serviceaccount.NewDeployer(log, wContext.Core.ServiceAccount()),
		clusterRoleBindingDeployer: clusterrolebinding.NewDeployer(log, wContext.RBAC.ClusterRoleBinding()),
		deploymentDeployer:         deployment.NewDeployer(log, wContext.Apps.Deployment()),
	}, nil
}

func (d *SCCDeployer) HasAllDependencies() (bool, error) {
	if hasNamespace, err := d.namespaceDeployer.HasResource(); err != nil || !hasNamespace {
		return false, err
	}

	if hasServiceAccount, err := d.serviceAccountDeployer.HasResource(); err != nil || !hasServiceAccount {
		return false, err
	}

	if hasClusterRB, err := d.clusterRoleBindingDeployer.HasResource(); err != nil || !hasClusterRB {
		return false, err
	}

	return true, nil
}

// EnsureDependenciesConfigured ensures that all dependencies for the SCC operator are configured
func (d *SCCDeployer) EnsureDependenciesConfigured(ctx context.Context, desiredParams *params.SCCOperatorParams) error {
	// Ensure namespace exists
	if err := d.namespaceDeployer.Ensure(ctx, desiredParams.Labels(params.TargetNamespace)); err != nil {
		return err
	}

	// Ensure the service account exists
	if err := d.serviceAccountDeployer.Ensure(ctx, desiredParams.Labels(params.TargetServiceAccount)); err != nil {
		return err
	}

	// Ensure the cluster role binding exists
	if err := d.clusterRoleBindingDeployer.Ensure(ctx, desiredParams.Labels(params.TargetClusterRoleBinding)); err != nil {
		return err
	}

	return nil
}

// EnsureSCCOperatorDeployed ensures that the SCC operator pod is deployed
func (d *SCCDeployer) EnsureSCCOperatorDeployed(ctx context.Context, desiredParams *params.SCCOperatorParams) error {
	d.deploymentDeployer.SetDesiredParams(desiredParams)
	return d.deploymentDeployer.Ensure(ctx, desiredParams.Labels(params.TargetDeployment))
}

func (d *SCCDeployer) ReconcileDeployment(ctx context.Context, desiredParams *params.SCCOperatorParams, incoming *appsv1.Deployment) (*appsv1.Deployment, error) {
	return d.deploymentDeployer.Reconcile(ctx, desiredParams, incoming)
}
