package namespace

import (
	"context"
	"fmt"
	"reflect"

	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer/types"
	"github.com/rancher/rancher/pkg/scc/util/generic"
	"github.com/rancher/rancher/pkg/scc/util/log"
)

// Deployer implements ResourceDeployer for Namespace resources
type Deployer struct {
	log        log.StructuredLogger
	namespaces v1core.NamespaceController
}

// NewDeployer creates a new NamespaceDeployer
func NewDeployer(log log.StructuredLogger, namespaces v1core.NamespaceController) *Deployer {
	return &Deployer{
		log:        log.WithField("deployer", "namespace"),
		namespaces: namespaces,
	}
}

func (d *Deployer) HasResource() (bool, error) {
	existing, err := d.namespaces.Get(consts.DefaultSCCNamespace, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return false, fmt.Errorf("error getting existing SCC namespace: %v", err)
	}

	return existing != nil, nil
}

// Ensure ensures that the SCC namespace exists and is not marked for deletion
func (d *Deployer) Ensure(ctx context.Context, labels map[string]string) error {
	// Check if namespace exists
	existingSccNs, err := d.namespaces.Get(consts.DefaultSCCNamespace, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for namespace %s: %w", consts.DefaultSCCNamespace, err)
	}

	desiredSccNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   consts.DefaultSCCNamespace,
			Labels: labels,
		},
	}

	// If namespace doesn't exist, create it
	if err != nil && errors.IsNotFound(err) {
		_, err = d.namespaces.Create(desiredSccNs)
		d.log.Infof("Created namespace: %s", consts.DefaultSCCNamespace)
		return err
	}

	// Check if namespace is marked for deletion
	if existingSccNs.DeletionTimestamp != nil {
		d.log.Infof("Namespace %s is marked for deletion, cleaning up and recreating", consts.DefaultSCCNamespace)

		// Try to remove finalizers to speed up deletion
		if len(existingSccNs.Finalizers) > 0 {
			d.log.Infof("Removing finalizers from namespace %s to speed up deletion", consts.DefaultSCCNamespace)
			existingSccNs.Finalizers = []string{}
			if _, err := d.namespaces.Update(existingSccNs); err != nil {
				d.log.Warnf("Failed to remove finalizers from namespace %s: %v", consts.DefaultSCCNamespace, err)
				// Continue anyway, we'll check if it's gone and recreate
			}
		}

		// Check if namespace is actually gone after removing finalizers
		_, err = d.namespaces.Get(consts.DefaultSCCNamespace, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			// Namespace is gone, create a new one
			d.log.Infof("Namespace %s is deleted, creating new one", consts.DefaultSCCNamespace)
			_, err = d.namespaces.Create(desiredSccNs)
			if err != nil {
				return fmt.Errorf("failed to create namespace %s after deletion: %w", consts.DefaultSCCNamespace, err)
			}
			d.log.Infof("Created namespace: %s after deletion", consts.DefaultSCCNamespace)
			return nil
		}

		// Namespace is still being deleted
		return fmt.Errorf("namespace %s is still being deleted, waiting for deletion to complete before recreating", consts.DefaultSCCNamespace)
	}

	// Update the namespace if needed
	patchUpdatedNs, patchCreateErr := generic.PreparePatchUpdated(existingSccNs, desiredSccNs)
	if patchCreateErr != nil {
		return patchCreateErr
	}

	// Check if update is needed
	if reflect.DeepEqual(existingSccNs, patchUpdatedNs) {
		d.log.Debugf("Namespace %s is up to date", consts.DefaultSCCNamespace)
		return nil
	}

	// Update the namespace
	d.log.Infof("Updating namespace: %s", consts.DefaultSCCNamespace)
	if _, err := d.namespaces.Update(patchUpdatedNs); err != nil {
		return fmt.Errorf("failed to update namespace %s: %w", consts.DefaultSCCNamespace, err)
	}

	d.log.Infof("Updated namespace: %s", consts.DefaultSCCNamespace)

	return nil
}

var _ types.ResourceDeployer = &Deployer{}
