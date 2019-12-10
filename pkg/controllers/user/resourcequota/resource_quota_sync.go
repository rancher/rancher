package resourcequota

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/types/convert"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	validate "github.com/rancher/rancher/pkg/resourcequota"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	NamespaceLister     v1.NamespaceLister
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

	projectLimit, _, err := getProjectResourceQuotaLimit(ns, c.ProjectLister)
	if err != nil {
		return ns, err
	}

	var quotaSpec *corev1.ResourceQuotaSpec
	if projectLimit != nil {
		quotaSpec, err = c.getNamespaceResourceQuota(ns)
		if err != nil {
			return ns, err
		}
	}

	quotaToUpdate, err := c.getResourceQuotaToUpdate(ns)
	if err != nil {
		return ns, err
	}

	operation := "none"
	if existing == nil {
		if quotaSpec != nil {
			operation = "create"
		}
	} else {
		if quotaSpec == nil {
			operation = "delete"
		} else if quotaToUpdate != "" || !apiequality.Semantic.DeepEqual(existing.Spec.Hard, quotaSpec.Hard) {
			operation = "update"
		}
	}

	var updated *corev1.Namespace
	var isFit bool
	switch operation {
	case "create":
		isFit, updated, err = c.validateAndSetNamespaceQuota(ns, quotaToUpdate)
		if err != nil {
			return updated, err
		}
		if !isFit {
			// create default "all 0" resource quota
			quotaSpec, err = getDefaultQuotaSpec()
			if err != nil {
				return updated, err
			}
		}
		err = c.createDefaultResourceQuota(ns, quotaSpec)
	case "update":
		isFit, updated, err = c.validateAndSetNamespaceQuota(ns, quotaToUpdate)
		if err != nil || !isFit {
			return updated, err
		}
		err = c.updateResourceQuota(existing, quotaSpec)
	case "delete":
		err = c.deleteResourceQuota(existing)
	}

	if updated == nil {
		updated = ns
	}

	if err != nil {
		return updated, err
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

func (c *SyncController) getNamespaceResourceQuota(ns *corev1.Namespace) (*corev1.ResourceQuotaSpec, error) {
	limit, err := getNamespaceResourceQuotaLimit(ns)
	if err != nil {
		return nil, err
	}
	if limit == nil {
		limit = defaultResourceLimit
	}

	return convertResourceLimitResourceQuotaSpec(limit)
}

func getDefaultQuotaSpec() (*corev1.ResourceQuotaSpec, error) {
	return convertResourceLimitResourceQuotaSpec(defaultResourceLimit)
}

var defaultResourceLimit = &v3.ResourceQuotaLimit{
	Pods:                   "0",
	Services:               "0",
	ReplicationControllers: "0",
	Secrets:                "0",
	ConfigMaps:             "0",
	PersistentVolumeClaims: "0",
	ServicesNodePorts:      "0",
	ServicesLoadBalancers:  "0",
	RequestsCPU:            "0",
	RequestsMemory:         "0",
	RequestsStorage:        "0",
	LimitsCPU:              "0",
	LimitsMemory:           "0",
}

func (c *SyncController) createDefaultResourceQuota(ns *corev1.Namespace, spec *corev1.ResourceQuotaSpec) error {
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

func (c *SyncController) validateAndSetNamespaceQuota(ns *corev1.Namespace, quotaToUpdate string) (bool, *corev1.Namespace, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return true, ns, nil
	}

	// get project limit
	projectLimit, projectID, err := getProjectResourceQuotaLimit(ns, c.ProjectLister)
	if err != nil {
		return false, ns, err
	}

	if projectLimit == nil {
		return true, ns, err
	}

	updatedNs := ns.DeepCopy()
	if quotaToUpdate != "" {
		if updatedNs.Annotations == nil {
			updatedNs.Annotations = map[string]string{}
		}
		updatedNs.Annotations[resourceQuotaAnnotation] = quotaToUpdate
		updatedNs, err = c.Namespaces.Update(updatedNs)
		if err != nil {
			return false, updatedNs, err
		}
	}

	// validate resource quota
	mu := validate.GetProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()
	// get other Namespaces
	objects, err := c.NsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return false, updatedNs, err
	}
	var nsLimits []*v3.ResourceQuotaLimit
	for _, o := range objects {
		other := o.(*corev1.Namespace)
		// skip itself
		if other.Name == ns.Name {
			continue
		}
		nsLimit, err := getNamespaceResourceQuotaLimit(other)
		if err != nil {
			return false, updatedNs, err
		}
		nsLimits = append(nsLimits, nsLimit)
	}
	nsLimit, err := getNamespaceResourceQuotaLimit(updatedNs)
	if err != nil {
		return false, updatedNs, err
	}
	isFit, msg, err := validate.IsQuotaFit(nsLimit, nsLimits, projectLimit)
	if err != nil {
		return false, updatedNs, err
	}

	if !isFit && msg != "" {
		msg = fmt.Sprintf("Resource quota [%v] exceeds project limit ", msg)
	}

	validated, err := c.setValidated(updatedNs, isFit, msg)

	return isFit, validated, err

}

func (c *SyncController) setValidated(ns *corev1.Namespace, value bool, msg string) (*corev1.Namespace, error) {
	set, err := namespaceutil.IsNamespaceConditionSet(ns, ResourceQuotaValidatedCondition, value)
	if set || err != nil {
		return ns, err
	}
	toUpdate := ns.DeepCopy()
	err = namespaceutil.SetNamespaceCondition(toUpdate, time.Second*1, ResourceQuotaValidatedCondition, value, msg)
	if err != nil {
		return ns, err
	}
	return c.Namespaces.Update(toUpdate)
}

func (c *SyncController) getResourceQuotaToUpdate(ns *corev1.Namespace) (string, error) {
	quota := getNamespaceResourceQuota(ns)
	defaultQuota, err := getProjectNamespaceDefaultQuota(ns, c.ProjectLister)
	if err != nil {
		return "", err
	}

	// rework after api framework change is done
	// when annotation field is passed as null, the annotation should be removed
	// instead of being updated with the null value
	var updatedQuota *v3.NamespaceResourceQuota
	if quota != "" && quota != "null" {
		// check if fields need to be removed or set
		// based on the default quota
		var existingQuota v3.NamespaceResourceQuota
		err := json.Unmarshal([]byte(convert.ToString(quota)), &existingQuota)
		if err != nil {
			return "", err
		}
		updatedQuota, err = completeQuota(&existingQuota, defaultQuota)
		if updatedQuota == nil || err != nil {
			return "", err
		}
	}

	var b []byte
	if updatedQuota == nil {
		b, err = json.Marshal(defaultQuota)
	} else {
		b, err = json.Marshal(updatedQuota)
	}

	if err != nil {
		return "", err
	}
	return string(b), nil
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
	var updatedLimit *v3.ContainerResourceLimit
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
	}

	return nil, nil
}

func completeQuota(existingQuota *v3.NamespaceResourceQuota, defaultQuota *v3.NamespaceResourceQuota) (*v3.NamespaceResourceQuota, error) {
	if defaultQuota == nil {
		return nil, nil
	}
	existingLimitMap, err := convert.EncodeToMap(existingQuota.Limit)
	if err != nil {
		return nil, err
	}
	newLimitMap, err := convert.EncodeToMap(defaultQuota.Limit)
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

	toReturn := existingQuota.DeepCopy()
	newLimit := v3.ResourceQuotaLimit{}
	if err := mapstructure.Decode(newLimitMap, &newLimit); err != nil {
		return nil, err
	}
	toReturn.Limit = newLimit
	return toReturn, nil
}

func completeLimit(existingLimit *v3.ContainerResourceLimit, defaultLimit *v3.ContainerResourceLimit) (*v3.ContainerResourceLimit, error) {
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

	newLimit := v3.ContainerResourceLimit{}
	if err := mapstructure.Decode(newLimitMap, &newLimit); err != nil {
		return nil, err
	}
	return &newLimit, nil
}
