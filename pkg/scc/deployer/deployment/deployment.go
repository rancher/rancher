package deployment

import (
	"context"
	"fmt"

	appsControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
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
	existingHash := fetchCurrentDeploymentHash(d.deployments)
	existingDeployment, getErr := d.deployments.Get(consts.DefaultSCCNamespace, consts.DeploymentName, metav1.GetOptions{})
	if getErr != nil && !errors.IsNotFound(getErr) {
		return fmt.Errorf("error checking for existing deployment %s/%s: %w", consts.DefaultSCCNamespace, consts.DeploymentName, getErr)
	}
	desiredDeployment := d.desiredParams.PrepareDeployment()

	if errors.IsNotFound(getErr) {
		_, createErr := d.deployments.Create(desiredDeployment)
		// todo conditionally wrap this for extra context when actual error
		return createErr
	}

	if d.desiredParams.RefreshHash != existingHash {
		d.log.Debug("The deployment hash has changed, so the deployment is in need of update.")
		patchUpdatedObj, err := generic.PreparePatchUpdated(existingDeployment, desiredDeployment)
		if err != nil {
			// todo: wrap this
			return err
		}

		_, err = d.deployments.Update(patchUpdatedObj)
		// todo conditionally wrap this for extra context when actual error
		return err
	}

	d.log.Debugf("The current deployment hash matched the desired one, so no update needed.")
	return nil
}

func (d *Deployer) SetDesiredParams(params *params.SCCOperatorParams) {
	d.desiredParams = params
}

var _ types.ResourceDeployer = &Deployer{}
