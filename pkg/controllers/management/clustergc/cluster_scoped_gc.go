package clustergc

import (
	"context"
	"strings"

	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
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
		obj, e := dynamicClient.Update(object, metav1.UpdateOptions{})
		return obj, e
	}
	return object, nil
}

func (c *gcLifecycle) Remove(cluster *v3.Cluster) (runtime.Object, error) {

	for key := range resource.Get() {
		objList, err := c.mgmt.DynamicClient.Resource(key).List(metav1.ListOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// skip this iteration, no objects were initialized of this type for the cluster
				continue
			}
			return cluster, err
		}

		for _, obj := range objList.Items {
			_, err = cleanFinalizers(cluster.Name, &obj, c.mgmt.DynamicClient.Resource(key).Namespace(obj.GetNamespace()))
			if err != nil {
				return cluster, err
			}
		}
	}

	return nil, nil
}
