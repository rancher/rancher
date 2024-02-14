package system

import (
	"context"

	v12 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DownstreamManager is an implementation of Manager that allows the upstream cluster to interact with the downstream one.
// It doesn't start any controller, nor loops. It DOESN'T ensure that the charts are kept up-to-date.
// Only the functions Ensure (used to install) and Manager.Uninstall should be used.
type DownstreamManager struct {
	Manager
}

// NewDownstreamManager can be used to create a DownstreamManager manually.
func NewDownstreamManager(ctx context.Context,
	contentManager ContentClient,
	ops OperationClient,
	pods corecontrollers.PodClient,
	helmClient HelmClient) (*DownstreamManager, error) {

	m := Manager{
		ctx:                   ctx,
		operation:             ops,
		content:               contentManager,
		pods:                  pods,
		sync:                  nil,
		desiredCharts:         nil,
		refreshIntervalChange: nil,
		settings:              nil,
		trigger:               nil,
		clusterRepos:          nil,
		helmClient:            helmClient,
	}

	return &DownstreamManager{m}, nil
}

// Start this function shouldn't be used, the DownstreamManager doesn't start controllers nor have the logic to keep
// the charts up to date. Use Ensure directly to install the charts.
func (d *DownstreamManager) Start(_ context.Context) {
	log.Error("The DownstreamManager shouldn't be started, calling ensure directly will install the required chart.")
	return
}

// Ensure differently of the Manager this function will call installCharts directly. It doesn't have any loop that
// will keep the chart up to date.
func (d *DownstreamManager) Ensure(namespace, name, minVersion, exactVersion string, values map[string]interface{}, forceAdopt bool, installImageOverride string) error {

	return d.installCharts(
		map[desiredKey]map[string]interface{}{
			desiredKey{
				namespace:            namespace,
				name:                 name,
				minVersion:           minVersion,
				exactVersion:         exactVersion,
				installImageOverride: installImageOverride,
			}: values,
		},
		forceAdopt,
	)
}

// Remove DownstreamManager don't have the concept of "required charts" therefore the remove function is useless
func (d *DownstreamManager) Remove(_, _ string) {
	log.Error("The DownstreamManager doesn't handle syncs, therefore we don't have a map of required maps, to uninstall use Uninstall")
	return
}

// TODO:
//   * Is it better to do one for each or to us generics?  If i use generics the "IDE" Complains about miss implementation, despite it working.
//   * Where should this interfaces be declared? Do we have any rule / recommendation about it?

// HelmNoNamespaceNoOptGetter allows a v1.ClusterRepoController to be used as a content.ClusterRepoNoNamespaceNoOptionGetter.
// It does so  by using the .Get with empty Options
type HelmNoNamespaceNoOptGetter struct {
	v1.ClusterRepoController
}

func (n HelmNoNamespaceNoOptGetter) Get(name string) (*v12.ClusterRepo, error) {
	return n.ClusterRepoController.Get(name, metav1.GetOptions{})
}

// ConfigMapNoOptGetter allows a ConfigMapClient to be used as a content.ConfigMapNoOptionGetter.
// It does so  by using the .Get with empty Options
type ConfigMapNoOptGetter struct {
	corecontrollers.ConfigMapClient
}

func (n ConfigMapNoOptGetter) Get(namespace, name string) (*corev1.ConfigMap, error) {
	return n.ConfigMapClient.Get(namespace, name, metav1.GetOptions{})
}

// SecretNoOptGetter allows a SecretClient to be used as a catalogv2.SecretGetterNoOption .
// It does so  by using the .Get with empty Options
type SecretNoOptGetter struct {
	corecontrollers.SecretClient
}

func (n SecretNoOptGetter) Get(namespace, name string) (*corev1.Secret, error) {
	return n.SecretClient.Get(namespace, name, metav1.GetOptions{})
}

// NoNamespaceNoOptGetter is a generic implementation that allows a NonNamespacedControllerInterface to be used as a
// ClusterRepoNoNamespaceNoOptionGetter TODO - VAlidate if I do generic or not here.
type NoNamespaceNoOptGetter[T generic.RuntimeMetaObject, TList runtime.Object] struct {
	generic.NonNamespacedControllerInterface[T, TList]
}

func (n NoNamespaceNoOptGetter[T, TList]) Get(name string) (T, error) {
	return n.NonNamespacedControllerInterface.Get(name, metav1.GetOptions{})
}

// NoOptGetter is an implementation tha allows a ClientInterface to be cast to a content.GenericNoOptionGetter
// It does it by calling hte Get with empty GetOptions. This does not implement any cache strategy.
type NoOptGetter[T generic.RuntimeMetaObject, TList runtime.Object] struct {
	generic.ClientInterface[T, TList]
}

func (n NoOptGetter[T, TList]) Get(namespace, name string) (T, error) {
	return n.ClientInterface.Get(namespace, name, metav1.GetOptions{})
}
