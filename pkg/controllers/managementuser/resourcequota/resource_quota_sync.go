package resourcequota

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	validate "github.com/rancher/rancher/pkg/resourcequota"
	"github.com/rancher/rancher/pkg/utils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientcache "k8s.io/client-go/tools/cache"
)

const (
	projectIDAnnotation             = "field.cattle.io/projectId"
	resourceQuotaLabel              = "resourcequota.management.cattle.io/default-resource-quota"
	resourceQuotaAnnotation         = "field.cattle.io/resourceQuota"
	limitRangeAnnotation            = "field.cattle.io/containerDefaultResourceLimit"
	ResourceQuotaValidatedCondition = "ResourceQuotaValidated"
	ResourceQuotaInitCondition      = "ResourceQuotaInit"
)

/*
SyncController takes care of creating Kubernetes resource quota based on the resource limits
defined in namespace.resourceQuota
*/
type SyncController struct {
	ProjectLister       v3.ProjectLister
	Namespaces          v1.NamespaceInterface
	ResourceQuotas      v1.ResourceQuotaInterface
	ResourceQuotaLister v1.ResourceQuotaLister
	LimitRange          v1.LimitRangeInterface
	LimitRangeLister    v1.LimitRangeLister
	NsIndexer           clientcache.Indexer
}

func (c *SyncController) syncResourceQuota(key string, ns *corev1.Namespace) (runtime.Object, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil, nil
	}

	_, err := c.CreateResourceQuota(ns)
	if err != nil {
		return nil, err
	}

	return nil, c.createLimitRange(ns)
}

func (c *SyncController) createLimitRange(ns *corev1.Namespace) error {
	existing, err := c.getExistingLimitRange(ns)
	if err != nil {
		return err
	}

	limitRangeSpec, err := c.getResourceLimitToUpdate(ns)
	if err != nil {
		return err
	}

	operation := "none"
	if existing == nil {
		if limitRangeSpec != nil {
			operation = "create"
		}
	} else {
		if limitRangeSpec == nil {
			operation = "delete"
		} else if limitsChanged(existing.Spec.Limits, limitRangeSpec.Limits) {
			operation = "update"
		}
	}

	switch operation {
	case "create":
		return c.createDefaultLimitRange(ns, limitRangeSpec)
	case "update":
		return c.updateDefaultLimitRange(existing, limitRangeSpec)
	case "delete":
		return c.deleteDefaultLimitRange(existing)
	}

	return nil
}

func limitsChanged(existing []corev1.LimitRangeItem, toUpdate []corev1.LimitRangeItem) bool {
	if len(existing) != len(toUpdate) {
		return true
	}
	if len(existing) == 0 || len(toUpdate) == 0 {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing[0].DefaultRequest, toUpdate[0].DefaultRequest) {
		return true
	}

	if !apiequality.Semantic.DeepEqual(existing[0].Default, toUpdate[0].Default) {
		return true
	}
	return false
}

func (c *SyncController) CreateResourceQuota(ns *corev1.Namespace) (runtime.Object, error) {
	existing, err := c.getExistingResourceQuota(ns)
	if err != nil {
		return ns, err
	}

	requestedQuotaLimit, newQuotaSpec, err := c.deriveRequestedResourceQuota(ns)
	if err != nil {
		return ns, err
	}

	operation := "none"
	if existing == nil {
		if newQuotaSpec != nil && len(newQuotaSpec.Hard) > 0 {
			operation = "create"
		}
	} else {
		if newQuotaSpec == nil || len(newQuotaSpec.Hard) == 0 {
			operation = "delete"
		} else if !apiequality.Semantic.DeepEqual(existing.Spec.Hard, newQuotaSpec.Hard) {
			operation = "update"
		}
	}

	var updated *corev1.Namespace
	var operationErr error
	switch operation {
	case "create":
		isFit, updated, exceeded, err := c.validateAndSetNamespaceQuota(ns, &v32.NamespaceResourceQuota{Limit: *requestedQuotaLimit})
		if err != nil {
			return updated, err
		}
		if !isFit {
			// Create a quota with zeros only for overused resources.
			limit, err := zeroOutResourceQuotaLimit(requestedQuotaLimit, exceeded)
			if err != nil {
				return updated, err
			}

			newQuotaSpec, err = convertResourceLimitResourceQuotaSpec(limit)
			if err != nil {
				return updated, err
			}
		}
		operationErr = c.createResourceQuota(ns, newQuotaSpec)
	case "update":
		isFit, upd, _, err := c.validateAndSetNamespaceQuota(ns, &v32.NamespaceResourceQuota{Limit: *requestedQuotaLimit})
		if err != nil {
			return upd, err
		}
		if !isFit {
			updated = upd
			break
		}
		operationErr = c.updateResourceQuota(existing, newQuotaSpec)
	case "delete":
		updatedNs := ns.DeepCopy()
		delete(updatedNs.Annotations, resourceQuotaAnnotation)
		updatedNs, err = c.Namespaces.Update(updatedNs)
		if err != nil {
			return updatedNs, err
		}
		operationErr = c.deleteResourceQuota(existing)
	}

	if updated == nil {
		updated = ns
	}

	if operationErr != nil {
		logrus.Errorf("Failed to perform operation %q on namespace %q: %v", operation, ns.Name, operationErr)
		return updated, operationErr
	}

	set, err := namespaceutil.IsNamespaceConditionSet(ns, ResourceQuotaInitCondition, true)
	if err != nil || set {
		return updated, err
	}
	toUpdate := updated.DeepCopy()
	namespaceutil.SetNamespaceCondition(toUpdate, time.Second*1, ResourceQuotaInitCondition, true, "")
	return c.Namespaces.Update(toUpdate)
}

func (c *SyncController) updateResourceQuota(quota *corev1.ResourceQuota, spec *corev1.ResourceQuotaSpec) error {
	toUpdate := quota.DeepCopy()
	toUpdate.Spec = *spec
	logrus.Infof("Updating default resource quota for namespace %v", toUpdate.Namespace)
	_, err := c.ResourceQuotas.Update(toUpdate)
	return err
}

func (c *SyncController) updateDefaultLimitRange(limitRange *corev1.LimitRange, spec *corev1.LimitRangeSpec) error {
	toUpdate := limitRange.DeepCopy()
	toUpdate.Spec = *spec
	logrus.Infof("Updating default limit range for namespace %v", toUpdate.Namespace)
	_, err := c.LimitRange.Update(toUpdate)
	return err
}

func (c *SyncController) deleteResourceQuota(quota *corev1.ResourceQuota) error {
	logrus.Infof("Deleting default resource quota for namespace %v", quota.Namespace)
	return c.ResourceQuotas.DeleteNamespaced(quota.Namespace, quota.Name, &metav1.DeleteOptions{})
}

func (c *SyncController) deleteDefaultLimitRange(limitRange *corev1.LimitRange) error {
	logrus.Infof("Deleting limit range %v for namespace %v", limitRange.Name, limitRange.Namespace)
	return c.LimitRange.DeleteNamespaced(limitRange.Namespace, limitRange.Name, &metav1.DeleteOptions{})
}

func (c *SyncController) getExistingResourceQuota(ns *corev1.Namespace) (*corev1.ResourceQuota, error) {
	set := labels.Set(map[string]string{resourceQuotaLabel: "true"})
	quota, err := c.ResourceQuotaLister.List(ns.Name, set.AsSelector())
	if err != nil {
		return nil, err
	}
	if len(quota) == 0 {
		return nil, nil
	}
	return quota[0], nil
}

func (c *SyncController) getExistingLimitRange(ns *corev1.Namespace) (*corev1.LimitRange, error) {
	set := labels.Set(map[string]string{resourceQuotaLabel: "true"})
	limitRanger, err := c.LimitRangeLister.List(ns.Name, set.AsSelector())
	if err != nil {
		return nil, err
	}
	if len(limitRanger) == 0 {
		return nil, nil
	}
	return limitRanger[0], nil
}

// deriveRequestedResourceQuota tries to obtain the new namespace's resource quota limit and its quota spec.
// It derives it by looking up the requested quota limit. If it's not found, then it looks up the project's default
// quota for a namespace. If it's also not found, then the method returns nil.
// If only the requested quota limit exists, then nil returned (no limits).
// If only the project's default namespace limit exists, then it is returned.
// If both exist, then the two limits are merged, with requested limits having priority for overlapping resources.
func (c *SyncController) deriveRequestedResourceQuota(ns *corev1.Namespace) (*v32.ResourceQuotaLimit, *corev1.ResourceQuotaSpec, error) {
	requested, err := getNamespaceResourceQuotaLimit(ns)
	if err != nil {
		return nil, nil, err
	}

	defaultQuota, err := getProjectNamespaceDefaultQuota(ns, c.ProjectLister)
	if err != nil {
		return nil, nil, err
	}

	var quotaLimit *v32.ResourceQuotaLimit

	if requested != nil && defaultQuota == nil {
		return nil, nil, nil
	} else if requested == nil && defaultQuota != nil {
		quotaLimit = &defaultQuota.Limit
	} else if requested != nil && defaultQuota != nil {
		quotaLimit, err = completeQuota(requested, &defaultQuota.Limit)
		if err != nil {
			return nil, nil, err
		}
	} else {
		// This use case arises when users create a namespace outside any projects.
		return nil, nil, nil
	}

	newQuotaSpec, err := convertResourceLimitResourceQuotaSpec(quotaLimit)
	if err != nil {
		return nil, nil, err
	}
	return quotaLimit, newQuotaSpec, nil
}

func (c *SyncController) createResourceQuota(ns *corev1.Namespace, spec *corev1.ResourceQuotaSpec) error {
	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "default-",
			Namespace:    ns.Name,
			Labels:       map[string]string{resourceQuotaLabel: "true"},
		},
		Spec: *spec,
	}
	logrus.Infof("Creating default resource quota for namespace %v", ns.Name)
	_, err := c.ResourceQuotas.Create(resourceQuota)
	return err
}

func (c *SyncController) createDefaultLimitRange(ns *corev1.Namespace, spec *corev1.LimitRangeSpec) error {
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "default-",
			Namespace:    ns.Name,
			Labels:       map[string]string{resourceQuotaLabel: "true"},
		},
		Spec: *spec,
	}
	logrus.Infof("Creating limit range %v for namespace %v", limitRange.Spec, ns.Name)
	_, err := c.LimitRange.Create(limitRange)
	return err
}

func (c *SyncController) validateAndSetNamespaceQuota(ns *corev1.Namespace, quotaToUpdate *v32.NamespaceResourceQuota) (bool, *corev1.Namespace, corev1.ResourceList, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return true, ns, nil, nil
	}

	// get project limit
	projectLimit, projectID, err := getProjectResourceQuotaLimit(ns, c.ProjectLister)
	if err != nil {
		return false, ns, nil, err
	}

	if projectLimit == nil {
		return true, ns, nil, err
	}

	updatedNs := ns.DeepCopy()
	if quotaToUpdate != nil {
		if updatedNs.Annotations == nil {
			updatedNs.Annotations = map[string]string{}
		}
		b, err := json.Marshal(quotaToUpdate)
		if err != nil {
			return false, ns, nil, err
		}
		updatedNs.Annotations[resourceQuotaAnnotation] = string(b)
		updatedNs, err = c.Namespaces.Update(updatedNs)
		if err != nil {
			return false, updatedNs, nil, err
		}
	}

	// validate resource quota
	mu := validate.GetProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	// Get other namespaces' limits.
	nsLimits, err := c.getNamespacesLimits(ns, projectID)
	if err != nil {
		return false, updatedNs, nil, err
	}
	isFit, exceeded, err := validate.IsQuotaFit(&quotaToUpdate.Limit, nsLimits, projectLimit)
	if err != nil {
		return false, updatedNs, nil, err
	}

	var msg string
	if !isFit && exceeded != nil {
		msg = fmt.Sprintf("Resource quota [%v] exceeds project limit", utils.FormatResourceList(exceeded))
	}

	validated, err := c.setValidated(updatedNs, isFit, msg)

	return isFit, validated, exceeded, err
}

func (c *SyncController) getNamespacesLimits(ns *v1.Namespace, projectID string) ([]*v32.ResourceQuotaLimit, error) {
	objects, err := c.NsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return nil, err
	}
	var nsLimits []*v32.ResourceQuotaLimit
	for _, o := range objects {
		other := o.(*corev1.Namespace)
		// Skip itself.
		if other.Name == ns.Name {
			continue
		}
		nsLimit, err := getNamespaceResourceQuotaLimit(other)
		if err != nil {
			return nil, err
		}
		nsLimits = append(nsLimits, nsLimit)
	}
	return nsLimits, nil
}

func (c *SyncController) setValidated(ns *corev1.Namespace, value bool, msg string) (*corev1.Namespace, error) {
	toUpdate := ns.DeepCopy()
	if err := namespaceutil.SetNamespaceCondition(toUpdate, time.Second*1, ResourceQuotaValidatedCondition, value, msg); err != nil {
		return ns, err
	}
	return c.Namespaces.Update(toUpdate)
}

func (c *SyncController) getResourceLimitToUpdate(ns *corev1.Namespace) (*corev1.LimitRangeSpec, error) {
	nsLimit, err := getNamespaceContainerResourceLimit(ns)
	if err != nil {
		return nil, err
	}
	projectLimit, err := getProjectContainerDefaultLimit(ns, c.ProjectLister)
	if err != nil {
		return nil, err
	}

	// rework after api framework change is done
	// when annotation field is passed as null, the annotation should be removed
	// instead of being updated with the null value
	var updatedLimit *v32.ContainerResourceLimit
	if nsLimit != nil {
		// check if fields need to be removed or set
		// based on the default quota
		updatedLimit, err = completeLimit(nsLimit, projectLimit)
		if err != nil {
			return nil, err
		}
	}

	if updatedLimit != nil {
		return convertPodResourceLimitToLimitRangeSpec(updatedLimit)
	} else if nsLimit != nil {
		return convertPodResourceLimitToLimitRangeSpec(nsLimit)
	} else if projectLimit != nil {
		return convertPodResourceLimitToLimitRangeSpec(projectLimit)
	} else {
		return nil, nil
	}
}

func completeQuota(requestedQuota *v32.ResourceQuotaLimit, defaultQuota *v32.ResourceQuotaLimit) (*v32.ResourceQuotaLimit, error) {
	if requestedQuota == nil || defaultQuota == nil {
		return nil, nil
	}
	requestedQuotaMap, err := convert.EncodeToMap(requestedQuota)
	if err != nil {
		return nil, err
	}
	newLimitMap, err := convert.EncodeToMap(defaultQuota)
	if err != nil {
		return nil, err
	}
	for key, value := range requestedQuotaMap {
		// Only override the values for keys (resources) that actually exist in the project quota.
		if newLimitMap[key] != nil {
			newLimitMap[key] = value
		}
	}

	toReturn := &v32.ResourceQuotaLimit{}
	err = convert.ToObj(newLimitMap, toReturn)
	return toReturn, err
}

func completeLimit(existingLimit *v32.ContainerResourceLimit, defaultLimit *v32.ContainerResourceLimit) (*v32.ContainerResourceLimit, error) {
	if defaultLimit == nil {
		return nil, nil
	}
	existingLimitMap, err := convert.EncodeToMap(existingLimit)
	if err != nil {
		return nil, err
	}
	newLimitMap, err := convert.EncodeToMap(defaultLimit)
	if err != nil {
		return nil, err
	}
	for key, value := range existingLimitMap {
		if _, ok := newLimitMap[key]; ok {
			newLimitMap[key] = value
		}
	}

	if reflect.DeepEqual(existingLimitMap, newLimitMap) {
		return nil, nil
	}

	newLimit := &v32.ContainerResourceLimit{}
	err = convert.ToObj(newLimitMap, newLimit)
	return newLimit, err
}

// zeroOutResourceQuotaLimit takes a resource quota limit and a list of resources exceeding the quota,
// and returns a new quota limit with exceeded resources zeroed out.
func zeroOutResourceQuotaLimit(limit *v32.ResourceQuotaLimit, exceeded corev1.ResourceList) (*v32.ResourceQuotaLimit, error) {
	limitMap, err := convert.EncodeToMap(limit)
	if err != nil {
		return nil, err
	}

	for k := range exceeded {
		resource := string(k)
		limitMap[resource] = "0"
	}

	toReturn := &v32.ResourceQuotaLimit{}
	err = convert.ToObj(limitMap, toReturn)
	return toReturn, err
}
