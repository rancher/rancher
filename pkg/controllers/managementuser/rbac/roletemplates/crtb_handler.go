package roletemplates

import (
	"errors"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

const (
	rtbOwnerLabel = "authz.cluster.cattle.io/rtb-owner-updated"
)

type crtbHandler struct {
	impersonationHandler *impersonationHandler
	crbClient            wrbacv1.ClusterRoleBindingController
	crtbCache            mgmtv3.ClusterRoleTemplateBindingCache
	crtbClient           mgmtv3.ClusterRoleTemplateBindingClient
	rtClient             mgmtv3.RoleTemplateController
	s                    *status.Status
	clusterName          string
}

func newCRTBHandler(uc *config.UserContext) (*crtbHandler, error) {
	impersonator, err := impersonation.ForCluster(uc)
	if err != nil {
		return nil, err
	}
	return &crtbHandler{
		impersonationHandler: &impersonationHandler{
			clusterName:  uc.ClusterName,
			impersonator: impersonator,
			crClient:     uc.RBACw.ClusterRole(),
			crtbCache:    uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
			prtbCache:    uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		},
		crbClient:   uc.RBACw.ClusterRoleBinding(),
		crtbCache:   uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		crtbClient:  uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		rtClient:    uc.Management.Wrangler.Mgmt.RoleTemplate(),
		s:           status.NewStatus(),
		clusterName: uc.ClusterName,
	}, nil
}

// OnChange ensures that the correct ClusterRoleBinding exists for the ClusterRoleTemplateBinding
func (c *crtbHandler) OnChange(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil || crtb.DeletionTimestamp != nil || !features.AggregatedRoleTemplates.Enabled() {
		return nil, nil
	}

	// Only run this controller if the CRTB is for this cluster
	if crtb.ClusterName != c.clusterName {
		return nil, nil
	}

	remoteConditions := []metav1.Condition{}
	if err := c.reconcileBindings(crtb, &remoteConditions); err != nil {
		return nil, errors.Join(err, c.updateStatus(crtb, remoteConditions))
	}

	// Ensure a service account impersonator exists on the cluster
	var err error
	if crtb.UserName != "" {
		err = c.impersonationHandler.ensureServiceAccountImpersonator(crtb.UserName)
		c.s.AddCondition(&remoteConditions, metav1.Condition{Type: ensureServiceAccountImpersonator}, serviceAccountImpersonatorExists, err)
	}

	return crtb, errors.Join(err, c.updateStatus(crtb, remoteConditions))
}

// reconcileBindings builds and creates ClusterRoleBinding for CRTB and removes any CRBs that shouldn't exist.
func (c *crtbHandler) reconcileBindings(crtb *v3.ClusterRoleTemplateBinding, remoteConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: reconcileClusterRoleBindings}

	isExternal, err := isRoleTemplateExternal(crtb.RoleTemplateName, c.rtClient)
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failureToGetRoleTemplate, err)
		return err
	}

	var roleName string
	if isExternal {
		roleName = crtb.RoleTemplateName
	} else {
		roleName = rbac.AggregatedClusterRoleNameFor(crtb.RoleTemplateName)
	}

	crb, err := rbac.BuildClusterRoleBindingFromRTB(crtb, roleName)
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failureToBuildClusterRoleBinding, err)
		return err
	}

	crb = AddAggregationFeatureLabel(crb).(*rbacv1.ClusterRoleBinding)

	labelSelector := labels.Set{
		rbac.CrtbOwnerLabel:     crtb.Name,
		AggregationFeatureLabel: "true",
	}
	aggregationCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: labelSelector.AsSelector().String()})
	if err != nil || aggregationCRBs == nil {
		c.s.AddCondition(remoteConditions, condition, failureToListClusterRoleBindings, err)
		return err
	}

	// Delete any ClusterRoleBindings that were created for this CRTB before the aggregation changes.
	// TODO: Remove this once roletemplate aggregation is the only enabled RBAC model. https://github.com/rancher/rancher/issues/53743
	legacyCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: rtbOwnerLabel + "=" + rbac.GetRTBLabel(crtb.ObjectMeta)})
	if err != nil || legacyCRBs == nil {
		c.s.AddCondition(remoteConditions, condition, failureToListClusterRoleBindings, err)
		return err
	}

	currentCRBs := append(aggregationCRBs.Items, legacyCRBs.Items...)

	// Find if the required CRB that already exists and delete all excess CRBs.
	// There should only ever be 1 cluster role binding per CRTB.
	var matchingCRB *rbacv1.ClusterRoleBinding
	for _, currentCRB := range currentCRBs {
		if rbac.IsClusterRoleBindingContentSame(crb, &currentCRB) && matchingCRB == nil {
			matchingCRB = &currentCRB
			continue
		}
		if err := rbac.DeleteResource(currentCRB.Name, c.crbClient); err != nil {
			c.s.AddCondition(remoteConditions, condition, failureToDeleteClusterRoleBinding, err)
			return err
		}
	}

	// If we didn't find an existing CRB, create it.
	if matchingCRB == nil {
		if _, err := c.crbClient.Create(crb); err != nil {
			c.s.AddCondition(remoteConditions, condition, failureToCreateClusterRoleBinding, err)
			return fmt.Errorf("failed to create cluster role binding %s: %w", crb.Name, err)
		}
	}
	c.s.AddCondition(remoteConditions, condition, clusterRoleBindingExists, nil)
	return nil
}

var timeNow = func() time.Time {
	return time.Now()
}

func (c *crtbHandler) updateStatus(crtb *v3.ClusterRoleTemplateBinding, remoteConditions []metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbFromCluster, err := c.crtbCache.Get(crtb.Namespace, crtb.Name)
		if err != nil {
			return err
		}
		if status.CompareConditions(crtbFromCluster.Status.RemoteConditions, remoteConditions) {
			return nil
		}

		crtbFromCluster.Status.SummaryRemote = status.SummaryCompleted
		if crtbFromCluster.Status.SummaryLocal == status.SummaryCompleted {
			crtbFromCluster.Status.Summary = status.SummaryCompleted
		}
		for _, c := range remoteConditions {
			if c.Status != metav1.ConditionTrue {
				crtbFromCluster.Status.Summary = status.SummaryError
				crtbFromCluster.Status.SummaryRemote = status.SummaryError
				break
			}
		}

		crtbFromCluster.Status.LastUpdateTime = timeNow().Format(time.RFC3339)
		crtbFromCluster.Status.ObservedGenerationRemote = crtb.ObjectMeta.Generation
		crtbFromCluster.Status.RemoteConditions = remoteConditions
		_, err = c.crtbClient.UpdateStatus(crtbFromCluster)
		if err != nil {
			return err
		}
		return nil
	})
}
