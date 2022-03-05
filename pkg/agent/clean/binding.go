// +build !windows

/*
Clean duplicates bindings found in a management cluster. This will collect all
PRTBs and CRTBs, create the labels used to identify the k8s resources that correspond
to those and check for duplicates. If they are found delete all but 1.
This is technically safe as rancher will recreate any CRB or RB that is deleted that
should not have been.
*/

package clean

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v32 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/pkg/generated/controllers/rbac"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	k8srbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

const (
	crtbType = "crtb"
	prtbType = "prtb"
)

type bindingsCleanup struct {
	crtbs               v3.ClusterRoleTemplateBindingClient
	prtbs               v3.ProjectRoleTemplateBindingClient
	clusterRoleBindings v1.ClusterRoleBindingClient
	roleBindings        v1.RoleBindingClient
}

func Bindings() error {
	logrus.Info("Starting bindings cleanup")
	if os.Getenv("DRY_RUN") == "true" {
		logrus.Info("DRY_RUN is true, no objects will be deleted/modified")
		dryRun = true
	}

	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		panic(err)
	}

	// No one wants to be slow
	config.RateLimiter = ratelimit.None

	rancherManagement, err := management.NewFactoryFromConfig(config)
	if err != nil {
		return err
	}

	k8srbac, err := rbac.NewFactoryFromConfig(config)
	if err != nil {
		return err
	}

	starters := []start.Starter{rancherManagement, k8srbac}

	ctx := context.Background()
	if err := start.All(ctx, 5, starters...); err != nil {
		return err
	}

	l := labelMigrator{
		crtbs:               rancherManagement.Management().V3().ClusterRoleTemplateBinding(),
		prtbs:               rancherManagement.Management().V3().ProjectRoleTemplateBinding(),
		clusterRoleBindings: k8srbac.Rbac().V1().ClusterRoleBinding(),
		roleBindings:        k8srbac.Rbac().V1().RoleBinding(),
	}

	if err := l.migrateLabels(); err != nil {
		return err
	}

	logrus.Info("migrated labels")

	bc := bindingsCleanup{
		crtbs:               rancherManagement.Management().V3().ClusterRoleTemplateBinding(),
		prtbs:               rancherManagement.Management().V3().ProjectRoleTemplateBinding(),
		clusterRoleBindings: k8srbac.Rbac().V1().ClusterRoleBinding(),
		roleBindings:        k8srbac.Rbac().V1().RoleBinding(),
	}

	return bc.clean()
}

func (bc *bindingsCleanup) clean() error {
	crtbs, err := bc.crtbs.List("", metav1.ListOptions{})
	if err != nil {
		return err
	}

	prtbs, err := bc.prtbs.List("", metav1.ListOptions{})
	if err != nil {
		return err
	}

	// The label's key and value changes depending on the rancher version
	var rancher25 bool

	// Check if we have the updated label, this indicates we are running on rancher 2.5+
	if len(crtbs.Items) > 0 {
		if _, ok := crtbs.Items[0].Labels[auth.RtbCrbRbLabelsUpdated]; ok {
			rancher25 = true
		}
	} else if len(prtbs.Items) > 0 {
		if _, ok := prtbs.Items[0].Labels[auth.RtbCrbRbLabelsUpdated]; ok {
			rancher25 = true
		}
	} else {
		logrus.Info("No clusterRoleTemplateBindings or projectRoleTemplateBindings found, exiting.")
		return nil
	}

	var waitGroup sync.WaitGroup

	waitGroup.Add(2)
	go func() {
		if err := bc.cleanCRTB(rancher25, crtbs.Items); err != nil {
			logrus.Error(err)
		}
		waitGroup.Done()
	}()

	go func() {
		if err := bc.cleanPRTB(rancher25, prtbs.Items); err != nil {
			logrus.Error(err)
		}
		waitGroup.Done()
	}()
	waitGroup.Wait()
	return nil
}

func (bc *bindingsCleanup) cleanCRTB(newLabel bool, crtbs []apiv3.ClusterRoleTemplateBinding) error {
	var objectMetas []metav1.ObjectMeta
	for _, crtb := range crtbs {
		objectMetas = append(objectMetas, crtb.ObjectMeta)
	}

	return bc.cleanObjectDuplicates(crtbType, newLabel, objectMetas)
}

func (bc *bindingsCleanup) cleanPRTB(newLabel bool, prtbs []apiv3.ProjectRoleTemplateBinding) error {
	var objectMetas []metav1.ObjectMeta
	for _, prtb := range prtbs {
		objectMetas = append(objectMetas, prtb.ObjectMeta)
	}

	return bc.cleanObjectDuplicates(prtbType, newLabel, objectMetas)
}

func (bc *bindingsCleanup) cleanObjectDuplicates(bindingType string, newLabel bool, objMetas []metav1.ObjectMeta) error {
	// Uppercase so the logging looks pretty
	bindingUpper := strings.ToUpper(bindingType)

	var returnErr error
	var totalCRBDupes, totalRoleDupes int

	for _, meta := range objMetas {
		labels := createLabelSelectors(newLabel, meta, bindingType)
		for _, label := range labels {
			var CRBduplicates, RBDupes int

			crbs, err := bc.clusterRoleBindings.List(metav1.ListOptions{LabelSelector: label})
			if err != nil {
				multierror.Append(returnErr, err)
			}

			if len(crbs.Items) > 1 {
				CRBduplicates += len(crbs.Items) - 1
				if err := bc.dedupeCRB(crbs.Items); err != nil {
					multierror.Append(returnErr, err)
				}
			}

			roleBindings, err := bc.roleBindings.List("", metav1.ListOptions{LabelSelector: label})
			if err != nil {
				multierror.Append(returnErr, err)
			}

			if len(roleBindings.Items) > 1 {
				roleDuplicates, err := bc.dedupeRB(roleBindings.Items)
				if err != nil {
					multierror.Append(returnErr, err)
				}
				RBDupes += roleDuplicates
			}
			if CRBduplicates > 0 || RBDupes > 0 {
				totalCRBDupes += CRBduplicates
				totalRoleDupes += RBDupes
				logrus.Infof("%v %v label:%v Duplicates: CRB:%v RB:%v", bindingUpper, meta.Name, label, CRBduplicates, RBDupes)
			}
		}
	}
	logrus.Infof("Total %v duplicate clusterRoleBindings %v, roleBindings %v", bindingUpper, totalCRBDupes, totalRoleDupes)
	return returnErr
}

func (bc *bindingsCleanup) dedupeCRB(bindings []k8srbacv1.ClusterRoleBinding) error {
	// Sort by creation timestamp so we keep the oldest
	sort.Sort(crbByCreation(bindings))

	// Leave the first one alone, we only need the duplicates
	duplicates := bindings[1:]

	for _, binding := range duplicates {
		if !dryRun {
			if err := bc.clusterRoleBindings.Delete(binding.Name, &metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("error attempting to delete CRB %v %v", binding.Name, err)
			}
		} else {
			logrus.Infof("DryRun enabled, clusterRoleBinding %v would be deleted", binding.Name)
		}
	}
	return nil
}

func (bc *bindingsCleanup) dedupeRB(roleBindings []k8srbacv1.RoleBinding) (int, error) {
	// roleBindings need to be sorted by namespace. The list gets all of the roleBindings
	// with the correct label but we do the processing here to limit the amount of API
	// calls this has to do. Sorting off namespace here is much faster than doing a
	// call per namespace per label (and gentler on the API).
	var duplicatesFound int

	bindingMap := make(map[string][]k8srbacv1.RoleBinding)
	for _, b := range roleBindings {
		bindingMap[b.Namespace] = append(bindingMap[b.Namespace], b)
	}

	for _, bindings := range bindingMap {
		// Sort by creation timestamp so we keep the oldest
		sort.Sort(roleBindingByCreation(bindings))
		// Leave the first one alone, we only need the duplicates
		duplicates := bindings[1:]
		for _, binding := range duplicates {
			duplicatesFound++
			if !dryRun {
				if err := bc.roleBindings.Delete(binding.Namespace, binding.Name, &metav1.DeleteOptions{}); err != nil {
					logrus.Errorf("error attempting to delete RB %v %v", binding.Name, err)
				}
			} else {
				logrus.Infof("DryRun enabled, roleBinding %v in namespace %v would be deleted", binding.Name, binding.Namespace)
			}
		}
	}
	return duplicatesFound, nil
}

// createLabelSelectors creates the labels required to list both clusterRoleBindings and
// roleBindings. See https://github.com/rancher/rancher/pull/28423#issue-468992149 for an explanation
// of the labels.
func createLabelSelectors(newLabel bool, obj metav1.ObjectMeta, objType string) []string {
	var labelSelectors []string
	var key string

	// newLabel determines if we are using the newer rancher 2.5 style labels
	if newLabel {
		key = pkgrbac.GetRTBLabel(obj)
		labelSelectors = append(labelSelectors, key+"="+auth.MembershipBindingOwner)
	} else {
		key = string(obj.UID)
		labelSelectors = append(labelSelectors, key+"="+auth.MembershipBindingOwnerLegacy)
	}

	switch objType {
	case crtbType:
		labelSelectors = append(labelSelectors, key+"="+auth.CrtbInProjectBindingOwner)
	case prtbType:
		labelSelectors = append(labelSelectors, key+"="+auth.PrtbInClusterBindingOwner)
	}

	return labelSelectors
}

type crbByCreation []k8srbacv1.ClusterRoleBinding

func (n crbByCreation) Len() int      { return len(n) }
func (n crbByCreation) Swap(i, j int) { n[i], n[j] = n[j], n[i] }

func (n crbByCreation) Less(i, j int) bool {
	s := n[i].ObjectMeta.CreationTimestamp
	t := n[j].ObjectMeta.CreationTimestamp
	return s.Before(&t)
}

type roleBindingByCreation []k8srbacv1.RoleBinding

func (n roleBindingByCreation) Len() int      { return len(n) }
func (n roleBindingByCreation) Swap(i, j int) { n[i], n[j] = n[j], n[i] }

func (n roleBindingByCreation) Less(i, j int) bool {
	s := n[i].ObjectMeta.CreationTimestamp
	t := n[j].ObjectMeta.CreationTimestamp
	return s.Before(&t)
}

const (
	rtbCrbRbLabelsUpdated = "authz.cluster.cattle.io/crb-rb-labels-updated"
	rtbOwnerLabel         = "authz.cluster.cattle.io/rtb-owner-updated"
	rtbLabelUpdated       = "authz.cluster.cattle.io/rtb-label-updated"
	rtbOwnerLabelLegacy   = "authz.cluster.cattle.io/rtb-owner"
	owner                 = "owner-user"
)

type labelMigrator struct {
	crtbs               v3.ClusterRoleTemplateBindingClient
	prtbs               v3.ProjectRoleTemplateBindingClient
	clusterRoleBindings v1.ClusterRoleBindingClient
	roleBindings        v1.RoleBindingClient
}

func (l labelMigrator) migrateLabels() error {
	crtbs, err := l.crtbs.List("", metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, crtb := range crtbs.Items {
		err := l.reconcileCRTBUserClusterLabels(&crtb)
		if err != nil {
			logrus.Infof("ERROR: %v", err)
		}
	}

	logrus.Infof("migrated labels for crtbs")

	prtbs, err := l.prtbs.List("", metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, prtb := range prtbs.Items {
		err := l.reconcilePRTBUserClusterLabels(&prtb)
		if err != nil {
			logrus.Infof("ERROR: %v", err)
		}
	}

	logrus.Infof("migrated labels for prtbs")

	return nil
}
func (l labelMigrator) reconcileCRTBUserClusterLabels(binding *v32.ClusterRoleTemplateBinding) error {
	/* Prior to 2.5, for every CRTB, following CRBs are created in the user clusters
		1. CRTB.UID is the label value for a CRB, authz.cluster.cattle.io/rtb-owner=CRTB.UID
	Using this labels, list the CRBs and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[rtbCrbRbLabelsUpdated] == "true" {
		return nil
	}

	var returnErr error
	set := labels.Set(map[string]string{rtbOwnerLabelLegacy: string(binding.UID)})
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel)
	userCRBs, err := l.clusterRoleBindings.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		return err
	}
	bindingValue := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, crb := range userCRBs.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := l.clusterRoleBindings.Get(crb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[rtbOwnerLabel] = bindingValue
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := l.clusterRoleBindings.Update(crbToUpdate)
			return err
		})
		if retryErr != nil {
			returnErr = multierror.Append(returnErr, retryErr)
		}
	}
	if returnErr != nil {
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbToUpdate, updateErr := l.crtbs.Get(binding.Namespace, binding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		if crtbToUpdate.Labels == nil {
			crtbToUpdate.Labels = make(map[string]string)
		}
		crtbToUpdate.Labels[rtbCrbRbLabelsUpdated] = "true"
		_, err := l.crtbs.Update(crtbToUpdate)
		return err
	})
	return retryErr
}

func (l labelMigrator) reconcilePRTBUserClusterLabels(binding *v32.ProjectRoleTemplateBinding) error {
	/* Prior to 2.5, for every PRTB, following CRBs are created in the user clusters
		1. PRTB.UID is the label key for a CRB, PRTB.UID=owner-user
		2. PRTB.UID is the label value for RBs with authz.cluster.cattle.io/rtb-owner: PRTB.UID
	Using this labels, list the CRBs and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[rtbCrbRbLabelsUpdated] == "true" {
		return nil
	}

	var returnErr error
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(binding.Namespace+"_"+binding.Name, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	set := labels.Set(map[string]string{string(binding.UID): owner})
	userCRBs, err := l.clusterRoleBindings.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		return err
	}
	bindingLabel := pkgrbac.GetRTBLabel(binding.ObjectMeta)

	for _, crb := range userCRBs.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := l.clusterRoleBindings.Get(crb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[bindingLabel] = owner
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := l.clusterRoleBindings.Update(crbToUpdate)
			return err
		})
		if retryErr != nil {
			returnErr = multierror.Append(returnErr, retryErr)
		}
	}

	reqUpdatedOwnerLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	set = map[string]string{rtbOwnerLabelLegacy: string(binding.UID)}
	rbs, err := l.roleBindings.List("", metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqUpdatedOwnerLabel).String()})
	if err != nil {
		return err
	}
	for _, rb := range rbs.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			rbToUpdate, updateErr := l.roleBindings.Get(rb.Namespace, rb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if rbToUpdate.Labels == nil {
				rbToUpdate.Labels = make(map[string]string)
			}
			rbToUpdate.Labels[rtbOwnerLabel] = bindingLabel
			rbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := l.roleBindings.Update(rbToUpdate)
			return err
		})
		if retryErr != nil {
			returnErr = multierror.Append(returnErr, retryErr)
		}
	}

	if returnErr != nil {
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbToUpdate, updateErr := l.prtbs.Get(binding.Namespace, binding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		if crtbToUpdate.Labels == nil {
			crtbToUpdate.Labels = make(map[string]string)
		}
		crtbToUpdate.Labels[rtbCrbRbLabelsUpdated] = "true"
		_, err := l.prtbs.Update(crtbToUpdate)
		return err
	})
	return retryErr
}
