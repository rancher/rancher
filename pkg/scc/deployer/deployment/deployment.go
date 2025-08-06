package deployment

import (
	"context"
	"fmt"

	appsControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer/params"
	"github.com/rancher/rancher/pkg/scc/deployer/types"
	"github.com/rancher/rancher/pkg/scc/util/generic"
	"github.com/rancher/rancher/pkg/scc/util/log"
)

type Deployer struct {
	log         log.StructuredLogger
	deployments appsControllers.DeploymentController

	desiredParams *params.SCCOperatorParams
}

func NewDeployer(log log.StructuredLogger, deployment appsControllers.DeploymentController) *Deployer {
	return &Deployer{
		log:         log,
		deployments: deployment,
	}
}

func (d *Deployer) HasResource() (bool, error) {
	existing, err := d.deployments.Get(consts.DefaultSCCNamespace, consts.DeploymentName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return false, fmt.Errorf("error getting existing deployment: %v", err)
	}

	return existing != nil, nil
}

func (d *Deployer) Ensure(ctx context.Context, _ map[string]string) error {
	// check if deployment exists
	existingDeployment, getErr := d.deployments.Get(consts.DefaultSCCNamespace, consts.DeploymentName, metav1.GetOptions{})
	if getErr != nil && !errors.IsNotFound(getErr) {
		return fmt.Errorf("error checking for existing deployment %s/%s: %w", consts.DefaultSCCNamespace, consts.DeploymentName, getErr)
	}

	_, err := d.Reconcile(ctx, d.desiredParams, existingDeployment)
	return err
}

func (d *Deployer) Reconcile(_ context.Context, desiredParams *params.SCCOperatorParams, incoming *appsv1.Deployment) (*appsv1.Deployment, error) {
	existingHash := fetchCurrentDeploymentHash(incoming)
	desiredDeployment := desiredParams.PrepareDeployment()

	if existingHash == "" {
		created, createErr := d.deployments.Create(desiredDeployment)
		if createErr != nil {
			return nil, fmt.Errorf("error creating deployment %s/%s: %w", consts.DefaultSCCNamespace, consts.DeploymentName, createErr)
		}
		return created, nil
	}

	if desiredParams.RefreshHash != existingHash {
		d.log.Debug("The deployment hash has changed; the deployment must be updated or redeployed.")
		patchUpdatedObj, err := generic.PreparePatchUpdated(incoming, desiredDeployment)
		if err != nil {
			// todo: wrap this
			return incoming, err
		}

		updated, err := d.deployments.Update(patchUpdatedObj)
		if err != nil {
			return nil, fmt.Errorf("error updating deployment %s/%s: %w", consts.DefaultSCCNamespace, consts.DeploymentName, err)
		}
		return updated, nil
	}

	d.log.Debugf("The current deployment hash and desired hash match; no update needed.")
	return incoming, nil
}

func (d *Deployer) SetDesiredParams(params *params.SCCOperatorParams) {
	d.desiredParams = params
}

var _ types.ResourceDeployer = &Deployer{}
