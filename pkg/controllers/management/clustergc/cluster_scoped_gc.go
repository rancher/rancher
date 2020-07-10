package clustergc

import (
	"context"
	"strings"

	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	gc := &gcLifecycle{
		mgmt: management,
	}

	management.Management.Clusters("").AddLifecycle(ctx, "cluster-scoped-gc", gc)
}

type gcLifecycle struct {
	mgmt *config.ManagementContext
}

func (c *gcLifecycle) Create(obj *v3.Cluster) (runtime.Object, error) {
	return obj, nil
}

func (c *gcLifecycle) Updated(obj *v3.Cluster) (runtime.Object, error) {
	return nil, nil
}

func cleanFinalizers(clusterName string, object *unstructured.Unstructured, dynamicClient dynamic.ResourceInterface) (*unstructured.Unstructured, error) {
	object = object.DeepCopy()
	modified := false
	md, err := meta.Accessor(object)
	if err != nil {
		return object, err
	}
	finalizers := md.GetFinalizers()
	for i := len(finalizers) - 1; i >= 0; i-- {
		f := finalizers[i]
		if strings.HasPrefix(f, lifecycle.ScopedFinalizerKey) && strings.HasSuffix(f, "_"+clusterName) {
			finalizers = append(finalizers[:i], finalizers[i+1:]...)
			modified = true
		}
	}

	if modified {
		md.SetFinalizers(finalizers)
		obj, e := dynamicClient.Update(context.TODO(), object, metav1.UpdateOptions{})
		return obj, e
	}
	return object, nil
}

// Remove check all objects that have had a cluster scoped finalizer added to them to ensure dangling finalizers do not
// remain on objects that no longer have handlers associated with them
func (c *gcLifecycle) Remove(cluster *v3.Cluster) (runtime.Object, error) {
	RESTconfig := c.mgmt.RESTConfig
	// due to the large number of api calls, temporary raise the burst limit in order to reduce client throttling
	RESTconfig.Burst = 25
	dynamicClient, err := dynamic.NewForConfig(&RESTconfig)
	if err != nil {
		return nil, err
	}
	decodedMap := resource.GetClusterScopedTypes()
	//if map is empty, fall back to checking all Rancher types
	if len(decodedMap) == 0 {
		decodedMap = resource.Get()
	}
	var g errgroup.Group

	for key := range decodedMap {
		actualKey := key // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			objList, err := dynamicClient.Resource(actualKey).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}
			for _, obj := range objList.Items {
				_, err = cleanFinalizers(cluster.Name, &obj, dynamicClient.Resource(actualKey).Namespace(obj.GetNamespace()))
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err = g.Wait(); err != nil {
		return nil, err
	}
	return nil, nil
}
