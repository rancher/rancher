package clustergc

import (
	"context"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"os"
	"strings"
	"time"

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
		logrus.Printf("We actually FOUND something! for obj: %v", obj)
		return obj, e
	}
	return object, nil
}

func (c *gcLifecycle) Remove(cluster *v3.Cluster) (runtime.Object, error) {
	config := c.mgmt.RESTConfig
	config.Burst = 100
	dynamicClient, err := dynamic.NewForConfig(&config)
	if err != nil {
		panic(err)
	}
	//dynamicClient := c.mgmt.DynamicClient

	// DEBUG CODE
	start := time.Now()
	fileName := "output-" + start.String() + ".log"
	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	decodedMap := resource.GetClusterScopedTypes()
	logrus.SetOutput(f)
	defer logrus.SetOutput(os.Stdout)
	logrus.Print(cluster.Name)
	for key, value := range decodedMap {
		logrus.Warnf("Map values: %v", key)
		if value == false {
			logrus.Errorf("Map value was false: %v %v", key, value)
		}

	}
	// ^^ DEBUG CODE
	//if map is empty, fall back to checking all Rancher types
	if len(decodedMap) == 0 {
		decodedMap = resource.Get()
	}
	var g errgroup.Group

	for key := range decodedMap {
		actualKey := key // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			logrus.Printf("Listing all resources of type: %s ", actualKey)
			apiTime := time.Now()
			objList, err := dynamicClient.Resource(actualKey).List(metav1.ListOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}
			logrus.Infof("api call took: %s for %s", time.Since(apiTime), actualKey)
			iterationTime := time.Now()
			for _, obj := range objList.Items {
				logrus.Printf("cleaning object: %v ", obj)
				_, err = cleanFinalizers(cluster.Name, &obj, dynamicClient.Resource(actualKey).Namespace(obj.GetNamespace()))
				if err != nil {
					return err
				}
			}
			logrus.Printf("time to iterate over the obj list: %s", time.Since(iterationTime))
			return nil
		})
	}
	logrus.Printf("Goroutines are cool")
	if err := g.Wait(); err != nil {
		logrus.Errorf("errgroup:", err)
		return nil, err
	}
	logrus.Printf("function took %s \n \n \n ", time.Since(start))
	return nil, nil
}
