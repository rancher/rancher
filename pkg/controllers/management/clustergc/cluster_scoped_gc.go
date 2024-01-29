package clustergc

import (
	"context"
	"strings"
	"time"

	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/generic"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

func (c *gcLifecycle) waitForNodeRemoval(cluster *v3.Cluster) error {
	if cluster.Status.Driver != v32.ClusterDriverRKE {
		return nil // not an rke1 node, no need to pause
	}

	nodes, err := c.mgmt.Management.Nodes("").Controller().Lister().List(cluster.Name, labels.Everything())
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	var waitForNodeDelete bool
	for _, n := range nodes {
		// trigger the deletion of node for a custom cluster
		if n.Status.NodeTemplateSpec == nil && n.DeletionTimestamp == nil {
			_ = c.mgmt.Management.Nodes(n.Namespace).Delete(n.Name, &metav1.DeleteOptions{})
			waitForNodeDelete = true
		}
	}
	if waitForNodeDelete {
		logrus.Debugf("[cluster-scoped-gc] custom cluster %s still has rke1 nodes, checking again in 15s", cluster.Name)
		c.mgmt.Management.Clusters("").Controller().EnqueueAfter(cluster.Namespace, cluster.Name, 15*time.Second)
		return generic.ErrSkip
	}

	return nil
}

// Remove check all objects that have had a cluster scoped finalizer added to them to ensure dangling finalizers do not
// remain on objects that no longer have handlers associated with them
func (c *gcLifecycle) Remove(cluster *v3.Cluster) (runtime.Object, error) {
	if err := c.waitForNodeRemoval(cluster); err != nil {
		return cluster, err // ErrSkip if we still need to wait
	}

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
