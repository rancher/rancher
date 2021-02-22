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
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/pkg/generated/controllers/rbac"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	k8srbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
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
