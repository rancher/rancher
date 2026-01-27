package resourcequota

import (
	"fmt"
	"reflect"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	validate "github.com/rancher/rancher/pkg/resourcequota"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	quota "k8s.io/apiserver/pkg/quota/v1"
	clientcache "k8s.io/client-go/tools/cache"
)

// resourceQuotaProjectID is the name of the annotation used to store the
// project the resource quota annotation is for. When a move is made its value
// lags behind the controlling project (See field.cattle.io/projectId), enabling
// move detection, giving us access to the old project, and enabling us to
// properly update the old project's used-limit information.
const resourceQuotaProjectID = "field.cattle.io/resourceQuotaProjectId"

// calculateLimitController is responsible for calculating the combined limit
// set on the project's Namespaces, and setting this information in the project
type calculateLimitController struct {
	projects    wmgmtv3.ProjectClient
	namespaces  corew.NamespaceClient
	nsIndexer   clientcache.Indexer
	clusterName string
}

func (c *calculateLimitController) calculateResourceQuotaUsed(_ string, ns *corev1.Namespace) (*corev1.Namespace, error) {
	if ns == nil {
		return nil, nil
	}
	projectNowID := getProjectID(ns)
	projectOldID := getQuotaProjectID(ns)

	// States and Actions -- See https://github.com/rancher/rancher/issues/53186#issuecomment-3777200253
	//
	// Id | Old | Now | Meaning                   | Action
	// -- | --- | --- | ------------------------- | ------
	// 1  | ""  | ""  | Not assigned to a project | nothing to do
	// 2  | ""  | P   | Just assigned to P        | set label to P                             (retrigger @5)
	// 3  | P   | ""  | Just unassigned from P    | compute used-limit for P, drop label       (retrigger @1)
	// 4  | P1  | P2  | Moved from P1 to P2       | compute used-limit for P1, set label to P2 (retrigger @5)
	// 5  | P   | P   | Some other change         | compute used-limit for P
	//
	//                   /<-A3- (3) <-unassign-\
	//                  V                       \
	// (nil) -create-> (1) --assign-> (2) -A2-> (5) -move-> (4)
	//                                           ^          /
	//                                            \<---A4--/

	// 1, 2
	if projectOldID == "" {
		if projectNowID == "" {
			// 1 not assigned
			return nil, nil
		}
		// 2 assigned
		updatedNs := ns.DeepCopy()
		setQuotaProjectID(updatedNs, projectNowID)
		return c.namespaces.Update(updatedNs)
	}
	// 3, 4, 5
	if projectNowID == "" {
		// 3 unassigned
		if err := c.calculateProjectResourceQuota(projectOldID); err != nil {
			logrus.Errorf("quota calculation failed for %q: %v", projectOldID, err)
			return nil, err
		}
		updatedNs := ns.DeepCopy()
		deleteQuotaProjectID(updatedNs)
		return c.namespaces.Update(updatedNs)
	}
	// 4, 5
	if projectOldID != projectNowID {
		// 4 move
		if err := c.calculateProjectResourceQuota(projectOldID); err != nil {
			logrus.Errorf("quota calculation failed for %q: %v", projectOldID, err)
			return nil, err
		}
		updatedNs := ns.DeepCopy()
		setQuotaProjectID(updatedNs, projectNowID)
		return c.namespaces.Update(updatedNs)
	}
	// 5 other
	if err := c.calculateProjectResourceQuota(projectNowID); err != nil {
		logrus.Errorf("quota calculation failed for %q: %v", projectNowID, err)
		return nil, err
	}
	return nil, nil
}

func setQuotaProjectID(ns *corev1.Namespace, newProjectID string) {
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	ns.Annotations[resourceQuotaProjectID] = newProjectID
}

func deleteQuotaProjectID(ns *corev1.Namespace) {
	if ns.Annotations == nil {
		return
	}
	delete(ns.Annotations, resourceQuotaProjectID)
}

func getQuotaProjectID(ns *corev1.Namespace) string {
	if ns.Annotations != nil {
		return ns.Annotations[resourceQuotaProjectID]
	}
	return ""
}

func (c *calculateLimitController) calculateResourceQuotaUsedProject(key string, p *apiv3.Project) (runtime.Object, error) {
	if p == nil || p.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, c.calculateProjectResourceQuota(fmt.Sprintf("%s:%s", c.clusterName, p.Name))
}

func (c *calculateLimitController) calculateProjectResourceQuota(projectID string) error {
	projectNamespace, projectName := ref.Parse(projectID)
	project, err := c.projects.Get(projectNamespace, projectName, metav1.GetOptions{})
	if err != nil || project.Spec.ResourceQuota == nil {
		if errors.IsNotFound(err) {
			// If Rancher is unaware of a project, we should ignore trying to calculate the project resource quota
			// A non-existent project is likely managed by another Rancher (e.g. Hosted Rancher)
			return nil
		}
		return err
	}

	namespaces, err := c.nsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return err
	}
	nssResourceList := corev1.ResourceList{}
	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		if ns.DeletionTimestamp != nil {
			continue
		}
		set, err := namespaceutil.IsNamespaceConditionSet(ns, ResourceQuotaValidatedCondition, true)
		if err != nil {
			return err
		}
		if !set {
			continue
		}
		nsLimit, err := getNamespaceResourceQuotaLimit(ns)
		if err != nil {
			return err
		}
		nsResourceList, err := validate.ConvertLimitToResourceList(nsLimit)
		if err != nil {
			return fmt.Errorf("parsing namespace quota limits: %w", err)
		}
		nssResourceList = quota.Add(nssResourceList, nsResourceList)
	}
	limit, err := convertResourceListToLimit(nssResourceList)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(project.Spec.ResourceQuota.UsedLimit, *limit) {
		return nil
	}

	toUpdate := project.DeepCopy()
	toUpdate.Spec.ResourceQuota.UsedLimit = *limit
	_, err = c.projects.Update(toUpdate)
	return err
}
