package deployer

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/runtime"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer/clusterrolebinding"
	"github.com/rancher/rancher/pkg/scc/deployer/deployment"
	"github.com/rancher/rancher/pkg/scc/deployer/namespace"
	"github.com/rancher/rancher/pkg/scc/deployer/params"
	"github.com/rancher/rancher/pkg/scc/deployer/serviceaccount"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
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

// OnRelatedSettings handles changes to related settings
func (d *SCCDeployer) OnRelatedSettings(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if setting, ok := obj.(*v3.Setting); ok {
		if setting.Name == settings.SCCOperatorImage.Name {
			return []relatedresource.Key{{
				Namespace: consts.DefaultSCCNamespace,
				Name:      "", // TODO: something to track current deployment name
			}}, nil
		}
	}

	return nil, nil
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
