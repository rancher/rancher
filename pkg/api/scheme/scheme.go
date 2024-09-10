package scheme

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	cluster "github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	management "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	project "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	provisioning "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rke "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	ui "github.com/rancher/rancher/pkg/apis/ui.cattle.io/v1"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	scalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	knetworkingv1 "k8s.io/api/networking/v1"
	knetworkingv1beta1 "k8s.io/api/networking/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

type FilteredScheme struct {
	*runtime.Scheme
	excludedGVKs map[schema.GroupVersionKind]struct{}
}

func (f *FilteredScheme) New(kind schema.GroupVersionKind) (runtime.Object, error) {
	if _, ok := f.excludedGVKs[kind]; ok {
		return nil, fmt.Errorf("%s is explicitly excluded from this scheme", kind.String())
	}

	return f.Scheme.New(kind)
}

func (f *FilteredScheme) ExcludeGVK(kind schema.GroupVersionKind) {
	f.excludedGVKs[kind] = struct{}{}
}

var Scheme *FilteredScheme

var builders = []runtime.SchemeBuilder{
	ui.SchemeBuilder,
	rke.SchemeBuilder,
	provisioning.SchemeBuilder,
	project.SchemeBuilder,
	management.SchemeBuilder,
	cluster.SchemeBuilder,
	catalog.SchemeBuilder,
	fleet.SchemeBuilder,
	monitoringv1.SchemeBuilder,
	monitoringv1alpha1.SchemeBuilder,
	planv1.SchemeBuilder,
	appsv1.SchemeBuilder,
	appsv1beta1.SchemeBuilder,
	scalingv2.SchemeBuilder,
	batchv1.SchemeBuilder,
	v1.SchemeBuilder,
	v1beta1.SchemeBuilder,
	extv1beta1.SchemeBuilder,
	knetworkingv1.SchemeBuilder,
	knetworkingv1beta1.SchemeBuilder,
	policyv1beta1.SchemeBuilder,
	rbacv1.SchemeBuilder,
	rbacv1beta1.SchemeBuilder,
	storagev1.SchemeBuilder,
	storagev1beta1.SchemeBuilder,
	apiregistrationv1.SchemeBuilder,
	apiregistrationv1beta1.SchemeBuilder,
}

func init() {
	Scheme = &FilteredScheme{
		Scheme:       runtime.NewScheme(),
		excludedGVKs: make(map[schema.GroupVersionKind]struct{}),
	}

	if err := addKnownTypes(Scheme.Scheme); err != nil {
		panic(err)
	}

	// Some resources in Norman have values that are not part of go struct associated with their GVK.
	// When Norman uses the appropriate go struct it will drop any field not on that go struct.
	// Solution: Do not inform Norman about the struct associated with some GVKs so that it will fall back to the
	// unstructured object and allow the missing fields to be included.
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("ClusterList"))
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("NodeTemplateList"))
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("ClusterAlertRuleList"))
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("ClusterAlertGroupList"))
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("ProjectAlertRuleList"))
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("ProjectAlertGroupList"))
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("ClusterMonitorGraphList"))
	Scheme.ExcludeGVK(management.SchemeGroupVersion.WithKind("ProjectMonitorGraphList"))

}

func addKnownTypes(scheme *runtime.Scheme) error {
	for _, builder := range builders {
		err := builder.AddToScheme(scheme)
		if err != nil {
			return err
		}
	}

	return nil
}
