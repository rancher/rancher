package deployer

import (
	"context"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch/v5"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	appsv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

type SCCDeployer struct {
	log log.StructuredLogger

	useDeployerOperator              bool
	currentParams                    *sccOperatorParams
	currentSCCOperatorDeploymentHash string

	namespaces          v1core.NamespaceController
	serviceAccounts     v1core.ServiceAccountController
	clusterRoleBindings v1.ClusterRoleBindingController
	deployments         appsv1.DeploymentController
}

func NewSCCDeployer(wContext *wrangler.Context, log log.StructuredLogger) (*SCCDeployer, error) {
	// Allow for disabling the deployer actually deploying the operator for dev mode (running scc-operator from IDE)
	useDeployerOperator := true
	if GetBuiltinDisabledEnv() {
		useDeployerOperator = false
	}

	deployerParams, err := extractSccOperatorParams()
	if err != nil {
		log.Errorf("Failed to extract SCC operator params: %v", err)
		return nil, err
	}
	log.Debugf("SCC operator params: %v", deployerParams)

	maybeCurrentDeploymentHash := fetchCurrentDeploymentHash(wContext.Apps.Deployment())
	log.Debugf("Current deployment hash: %s", maybeCurrentDeploymentHash)

	return &SCCDeployer{
		log:                              log,
		useDeployerOperator:              useDeployerOperator,
		currentParams:                    deployerParams,
		currentSCCOperatorDeploymentHash: maybeCurrentDeploymentHash,
		namespaces:                       wContext.Core.Namespace(),
		serviceAccounts:                  wContext.Core.ServiceAccount(),
		clusterRoleBindings:              wContext.RBAC.ClusterRoleBinding(),
		deployments:                      wContext.Apps.Deployment(),
	}, nil
}

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

// ensureNamespace ensures that the SCC namespace exists
func (d *SCCDeployer) ensureNamespace(_ context.Context) error {
	// TODO: should we do some sort of "take over" if it already exists but wasn't created by us?
	existingSccNs, err := d.namespaces.Get(consts.DefaultSCCNamespace, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for namespace %s: %w", consts.DefaultSCCNamespace, err)
	}

	desiredSccNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: consts.DefaultSCCNamespace,
			Labels: map[string]string{
				consts.LabelK8sManagedBy: "rancher",
			},
		},
	}

	if err != nil && errors.IsNotFound(err) {
		_, err = d.namespaces.Create(desiredSccNs)
		d.log.Infof("Created namespace: %s", consts.DefaultSCCNamespace)
		return err
	}

	patchUpdatedNs, patchCreateErr := createNamespaceUpdatePatch(existingSccNs, desiredSccNs)
	if patchCreateErr != nil {
		return patchCreateErr
	}

	if existingSccNs == patchUpdatedNs {
		d.log.Debugf("namespace %s is up to date", consts.DefaultSCCNamespace)
		return nil
	}

	if _, err := d.namespaces.Update(patchUpdatedNs); err != nil {
		return fmt.Errorf("failed to update namespace %s: %w", consts.DefaultSCCNamespace, err)
	}

	d.log.Infof("Updated namespace: %s", consts.DefaultSCCNamespace)

	return nil
}

func createNamespaceUpdatePatch(incoming, desired *corev1.Namespace) (*corev1.Namespace, error) {
	incomingJson, err := json.Marshal(incoming)
	if err != nil {
		return nil, err
	}
	newJson, err := json.Marshal(desired)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreateMergePatch(incomingJson, newJson)
	if err != nil {
		return nil, err
	}

	updatedJson, err := jsonpatch.MergePatch(incomingJson, patch)
	if err != nil {
		return nil, err
	}

	var updatedObj corev1.Namespace
	if err := json.Unmarshal(updatedJson, &updatedObj); err != nil {
		return nil, err
	}

	return &updatedObj, nil
}

func (d *SCCDeployer) ensureServiceAccount(_ context.Context) error {
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
				Labels:    d.currentParams.Labels(),
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

func (d *SCCDeployer) ensureClusterRoleBinding(_ context.Context) error {
	_, err := d.clusterRoleBindings.Get("rancher-scc-operator", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for cluster role binding %s: %w", "rancher-scc-operator", err)
	}

	if errors.IsNotFound(err) {
		// ClusterRoleBinding to give the service account cluster-admin
		desiredClusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "rancher-scc-operator",
				Labels: map[string]string{
					consts.LabelK8sManagedBy: "rancher",
					consts.LabelK8sPartOf:    "scc-operator",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: consts.DefaultSCCNamespace,
					Name:      consts.ServiceAccountName,
				},
			},
		}

		_, err = d.clusterRoleBindings.Create(desiredClusterRoleBinding)
		if err != nil {
			return fmt.Errorf("error creating cluster role binding %s: %w", "rancher-scc-operator", err)
		}
	}

	return nil
}

func (d *SCCDeployer) EnsureDependenciesConfigured(ctx context.Context) error {
	if err := d.ensureNamespace(ctx); err != nil {
		return err
	}

	if err := d.ensureServiceAccount(ctx); err != nil {
		return err
	}

	if err := d.ensureClusterRoleBinding(ctx); err != nil {
		return err
	}

	return nil
}

func (d *SCCDeployer) EnsureSCCOperatorDeployment(ctx context.Context) error {
	if !d.useDeployerOperator {
		// TODO: update this.
		// Do nothing and/or scale to 0?
		return nil
	}

	return nil
}
