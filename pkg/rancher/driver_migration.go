package rancher

import (
	"fmt"

	"github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	linodeDriver = management.Linodedriver

	// One-shot ConfigMap marker: gates the "disable linode if unused" evaluation
	// so it runs only on the first Rancher start, not on every subsequent restart.
	linodeDisableCheckConfigMap = "disableunusedlinodenodedriver"
	linodeDisableCheckKey       = "unusedLinodeNodeDriverDisabled"

	linodeMachineConfigKind = "LinodeConfig"

	// DynamicSchema object names created by the nodedriver lifecycle for linode.
	linodeConfigSchemaName           = "linodeconfig"
	linodeCredentialConfigSchemaName = "linodecredentialconfig"

	// Parent DynamicSchema objects that embed the linode config fields.
	linodeParentNodeConfig   = "nodeconfig"
	linodeParentNodeTemplate = "nodetemplateconfig"
	linodeParentCredential   = "credentialconfig"

	// Fields in the parent DynamicSchema objects that embed the linode config fields.
	linodeConfigField     = "linodeConfig"
	linodeCredentialField = "linodecredentialConfig"

	// CRD names generated from the linodeconfig DynamicSchema.
	linodeConfigCRD          = "linodeconfigs.rke-machine-config.cattle.io"
	linodeMachineCRD         = "linodemachines.rke-machine.cattle.io"
	linodeMachineTemplateCRD = "linodemachinetemplates.rke-machine.cattle.io"
)

// disableUnusedLinodeNodeDriver runs once per Rancher installation (gated by a
// ConfigMap marker in cattle-system) to disable the linode NodeDriver and
// remove its associated DynamicSchema objects, parent-schema fields, and
// generated CRDs when no provisioning cluster is using it.
//
// This function MUST be called early in rancher.New(), before the wrangler
// SharedCacheFactory and lasso dynamic controller start (i.e. before
// MultiClusterManager.Start / steveserver.New). The reason is that once lasso
// starts it caches the RESTMappings of the linode CRDs in an internal
// memory-cache that is never invalidated on CRD deletion, so deleting the CRDs
// after that point causes lasso to keep resurrecting watchers for the deleted
// GVKs, which then 404 forever. Deleting the CRDs before lasso ever sees them
// ensures the mappings are never cached and no watchers are started.
//
// All operations use direct API calls (not informer caches) because this
// function runs before any informer is synced. All deletes and updates tolerate
// NotFound, so the function is idempotent and safe on fresh installs where the
// driver and its CRDs do not yet exist.
func disableUnusedLinodeNodeDriver(w *wrangler.Context) error {
	cm, err := w.Core.ConfigMap().Get(cattleNamespace, linodeDisableCheckConfigMap, v1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("DisableUnusedLinodeNodeDriver: failed to get marker ConfigMap: %w", err)
	}
	if apierrors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      linodeDisableCheckConfigMap,
				Namespace: cattleNamespace,
			},
			Data: make(map[string]string, 1),
		}
	}

	// Marker already set on a previous start: nothing to do.
	if cm.Data[linodeDisableCheckKey] == "true" {
		return nil
	}

	inUse, err := linodeNodeDriverInUse(w)
	if err != nil {
		return err
	}

	if inUse {
		logrus.Info("disableUnusedLinodeNodeDriver: linode node driver is in use by one or more provisioning clusters; leaving state unchanged")
	} else {
		logrus.Info("disableUnusedLinodeNodeDriver: deactivating the unused linode node driver")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// 1. Deactivate the NodeDriver CR.
			nd, err := w.Mgmt.NodeDriver().Get(linodeDriver, v1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if !nd.Spec.Active {
				return nil // Already inactive.
			}
			nd = nd.DeepCopy()
			nd.Spec.Active = false
			if _, err := w.Mgmt.NodeDriver().Update(nd); err != nil {
				return fmt.Errorf("failed to update NodeDriver: %w", err)
			}
			logrus.Debug("disableUnusedLinodeNodeDriver: deactivated nodeDriver linode")

			// 2. Delete the linodeconfig and linodecredentialconfig DynamicSchema
			//    objects. These are the source objects that the CAPR dynamicschema
			//    controller uses to generate the rke-machine CRDs; deleting them
			//    prevents any future re-generation.
			for _, schemaName := range []string{linodeConfigSchemaName, linodeCredentialConfigSchemaName} {
				if err := w.Mgmt.DynamicSchema().Delete(schemaName, &v1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete DynamicSchema %s: %w", schemaName, err)
				}
				logrus.Debugf("disableUnusedLinodeNodeDriver: deleted DynamicSchema %s", schemaName)
			}

			// 3. Remove the linode config fields from the parent schemas so that
			//    getStyle() can never return node=true for linode, even if the
			//    DynamicSchema objects are re-created.
			parentFields := []struct{ schema, field string }{
				{linodeParentNodeConfig, linodeConfigField},
				{linodeParentNodeTemplate, linodeConfigField},
				{linodeParentCredential, linodeCredentialField},
			}
			for _, pf := range parentFields {
				if err := removeFieldFromSchema(w, pf.schema, pf.field); err != nil {
					return fmt.Errorf("failed to remove field %s from schema %s: %w", pf.field, pf.schema, err)
				}
				logrus.Debugf("disableUnusedLinodeNodeDriver: disabled %s from schema %s", pf.field, pf.schema)
			}

			// 4. Delete the three CRDs generated from the linodeconfig DynamicSchema.
			//    This must happen before lasso's dynamic controller starts so the CRD
			//    RESTMappings are never cached in lasso's internal memory-cache mapper.
			for _, crdName := range []string{linodeConfigCRD, linodeMachineCRD, linodeMachineTemplateCRD} {
				if err := w.CRD.CustomResourceDefinition().Delete(crdName, &v1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete CRD %s: %w", crdName, err)
				}
				logrus.Debugf("disableUnusedLinodeNodeDriver: deleted CRD %s", crdName)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("disableUnusedLinodeNodeDriver: failed to deactivate linode NodeDriver: %w", err)
		}
	}

	// Persist the marker regardless of in-use status so the evaluation never
	// re-runs on subsequent starts (admin's choices are preserved thereafter).
	if cm.Data == nil {
		cm.Data = make(map[string]string, 1)
	}
	cm.Data[linodeDisableCheckKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

// linodeNodeDriverInUse reports whether any provisioning.cattle.io/v1 Cluster
// has a MachinePool whose machineConfigRef matches the linode node driver.
func linodeNodeDriverInUse(w *wrangler.Context) (bool, error) {
	clusters, err := w.Provisioning.Cluster().List("", v1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("linodeNodeDriverInUse: failed to list provisioning clusters: %w", err)
	}
	for _, cluster := range clusters.Items {
		if cluster.Spec.RKEConfig == nil {
			continue
		}
		for _, pool := range cluster.Spec.RKEConfig.MachinePools {
			// APIVersion is empty when the cluster is created via UI, so we don't check it here.
			if pool.NodeConfig != nil && pool.NodeConfig.Kind == linodeMachineConfigKind {
				return true, nil
			}
		}
	}
	return false, nil
}

// removeFieldFromSchema removes the named field from the ResourceFields of the
// specified DynamicSchema. Tolerates NotFound (schema does not exist yet) and
// is a no-op when the field is already absent.
func removeFieldFromSchema(w *wrangler.Context, schemaName, fieldName string) error {
	schema, err := w.Mgmt.DynamicSchema().Get(schemaName, v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, ok := schema.Spec.ResourceFields[fieldName]; !ok {
		return nil // Field already absent.
	}
	schema = schema.DeepCopy()
	delete(schema.Spec.ResourceFields, fieldName)
	_, err = w.Mgmt.DynamicSchema().Update(schema)
	return err
}
