package serviceaccount

import (
	"fmt"

	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer/types"
	"github.com/rancher/rancher/pkg/scc/util/log"

	"context"

	corev1 "k8s.io/api/core/v1"
)

// Deployer implements ResourceDeployer for ServiceAccount resources
type Deployer struct {
	log             log.StructuredLogger
	serviceAccounts v1core.ServiceAccountController
}

// NewDeployer creates a new ServiceAccountDeployer
func NewDeployer(
	log log.StructuredLogger,
	serviceAccounts v1core.ServiceAccountController,
) *Deployer {
	return &Deployer{
		log:             log.WithField("deployer", "serviceaccount"),
		serviceAccounts: serviceAccounts,
	}
}

func (d *Deployer) HasResource() (bool, error) {
	existing, err := d.serviceAccounts.Get(consts.DefaultSCCNamespace, consts.ServiceAccountName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return false, fmt.Errorf("error getting existing service account: %v", err)
	}

	return existing != nil, nil
}

// Ensure ensures that the service account exists and is configured correctly
func (d *Deployer) Ensure(ctx context.Context, labels map[string]string) error {
	saName := consts.ServiceAccountName
	_, err := d.serviceAccounts.Get(consts.DefaultSCCNamespace, saName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for service account %s in namespace %s: %w",
			saName, consts.DefaultSCCNamespace, err)
	}

	if errors.IsNotFound(err) {
		// Create the service account if it doesn't exist
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: consts.DefaultSCCNamespace,
				Labels:    labels,
			},
		}

		_, err = d.serviceAccounts.Create(sa)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				d.log.Debugf("Noop: service-account already existed")
				return nil
			}
			return fmt.Errorf("failed to create service account %s in namespace %s: %w",
				saName, consts.DefaultSCCNamespace, err)
		}
		d.log.Infof("Created service account: %s in namespace: %s", saName, consts.DefaultSCCNamespace)
	}

	return nil
}

var _ types.ResourceDeployer = &Deployer{}
