//go:build !windows
// +build !windows

/*
Clean orphaned bindings found in cluster namespaces. This will look for orphaned RoleBinding resources
in cluster namespaces and subsequently delete any that are found.
*/

package clean

import (
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/sirupsen/logrus"
	k8srbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	orphanBindingsOperation = "clean-orphan-bindings"
)

type orphanBindingsCleanup struct {
	prtbs        v3.ProjectRoleTemplateBindingInterface
	roleBindings rbacv1.RoleBindingInterface
	prtbUIDs     map[string]struct{}
}

func OrphanBindings(clientConfig *rest.Config) error {
	if os.Getenv("DRY_RUN") == "true" {
		logrus.Infof("[%v] DRY_RUN is true, no objects will be deleted/modified", orphanBindingsOperation)
		dryRun = true
	}

	var config *rest.Config
	var err error
	if clientConfig != nil {
		config = clientConfig
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			logrus.Errorf("[%v] Error in building the cluster config %v", orphanBindingsOperation, err)
			return err
		}
	}
	config.RateLimiter = ratelimit.None

	rancherManagement, err := v3.NewForConfig(*config)
	if err != nil {
		return err
	}

	k8srbac, err := rbacv1.NewForConfig(*config)
	if err != nil {
		return err
	}

	bc := orphanBindingsCleanup{
		prtbs:        rancherManagement.ProjectRoleTemplateBindings(""),
		prtbUIDs:     make(map[string]struct{}),
		roleBindings: k8srbac.RoleBindings(""),
	}

	logrus.Infof("[%v] cleaning up orphaned bindings", orphanBindingsOperation)
	return bc.cleanOrphans()
}

// cleanOrphans finds and deletes orphaned bindings
func (bc *orphanBindingsCleanup) cleanOrphans() error {
	logrus.Infof("[%v] checking for orphaned rolebindings", orphanBindingsOperation)

	// build the prtb uid map for checking existence of rb owner in legacy label case
	prtbs, err := bc.prtbs.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, prtb := range prtbs.Items {
		bc.prtbUIDs[string(prtb.UID)] = struct{}{}
	}

	// check all rolebindings against orphan criteria
	rbs, err := bc.roleBindings.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var returnErr error
	for _, rb := range rbs.Items {
		isOrphan, err := bc.isOrphanBinding(rb)
		if err != nil {
			returnErr = multierror.Append(returnErr, err)
		}
		if isOrphan {
			logrus.Infof("[%v] found orphaned binding: %s/%s", orphanBindingsOperation, rb.Namespace, rb.Name)
			if dryRun {
				logrus.Infof("[%v] dryRun is enabled, skipping deletion for orphaned binding: %s/%s", orphanBindingsOperation, rb.Namespace, rb.Name)
				continue
			}
			logrus.Infof("[%v] deleting orphaned binding: %s/%s", orphanBindingsOperation, rb.Namespace, rb.Name)
			err := bc.roleBindings.DeleteNamespaced(rb.Namespace, rb.Name, &metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				returnErr = multierror.Append(returnErr, err)
			}
		}
	}

	return returnErr
}

// isOrphanBinding detects whether or not rb is an orphaned binding. Only bindings with a Group subject may be orphans.
// To detect orphans, we look at labels with value == PrtbInClusterBindingOwner. If the key for this label is a UID,
// then it is considered to be legacy as the format for the key on this label was changed with 2.5. If the label's key is of the
// form <ns>_<prtb-name> then label is considered to be new (post 2.5.0). If we find a binding with a legacy label and not a new label, we check if there is an
// existing prtb with that uid. If there is not, then the binding is an orphan. If a new label exists on the binding, then we check if the parent
// prtb exists. If it does not exist, the binding is an orphan.
func (bc *orphanBindingsCleanup) isOrphanBinding(rb k8srbacv1.RoleBinding) (bool, error) {
	var hasGroupSubject bool
	if len(rb.Subjects) == 1 && rb.Subjects[0].Kind == k8srbacv1.GroupKind {
		hasGroupSubject = true
	}
	if !hasGroupSubject {
		return false, nil
	}

	var prtbNs, prtbName, uid string
	var foundLabel, hasNewLabel bool
	for k, v := range rb.Labels {
		if v != auth.PrtbInClusterBindingOwner {
			continue
		}
		foundLabel = true
		if strings.Contains(k, "_") { // only new label will have '_'
			hasNewLabel = true
			pieces := strings.Split(k, "_")
			prtbNs, prtbName = pieces[0], pieces[1]
		} else {
			uid = k
		}
	}

	// if there are no matching labels, we don't need to consider this binding
	if !foundLabel {
		return false, nil
	}

	if hasNewLabel {
		_, err := bc.prtbs.GetNamespaced(prtbNs, prtbName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return true, nil
			}
		}
		return false, err
	}

	// label is found but is not new, so it must be the legacy label
	if _, ok := bc.prtbUIDs[uid]; !ok {
		return true, nil // no prtb with the uid exists, so binding is orphaned
	}

	return false, nil // we have a prtb with this uid, so the binding is not an orphan
}
