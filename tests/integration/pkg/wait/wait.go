package wait

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type WatchFunc func(namespace string, opts metav1.ListOptions) (watch.Interface, error)
type WatchClusterScopedFunc func(opts metav1.ListOptions) (watch.Interface, error)

func ClusterScopedList(ctx context.Context, watchFunc WatchClusterScopedFunc, cb func(obj runtime.Object) (bool, error)) error {
	result, err := watchFunc(metav1.ListOptions{
		TypeMeta:       metav1.TypeMeta{},
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		result.Stop()
		for range result.ResultChan() {
		}
	}()

	for event := range result.ResultChan() {
		switch event.Type {
		case watch.Added:
			fallthrough
		case watch.Modified:
			fallthrough
		case watch.Deleted:
			done, err := cb(event.Object)
			if err != nil || done {
				return err
			}
		}
	}

	return nil
}

func Object(ctx context.Context, watchFunc WatchFunc, obj runtime.Object, cb func(obj runtime.Object) (bool, error)) error {
	if done, err := cb(obj); err != nil || done {
		return err
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	result, err := watchFunc(meta.GetNamespace(), metav1.ListOptions{
		TypeMeta:        metav1.TypeMeta{},
		FieldSelector:   "metadata.name=" + meta.GetName(),
		ResourceVersion: meta.GetResourceVersion(),
		TimeoutSeconds:  &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		result.Stop()
		for range result.ResultChan() {
		}
	}()

	var last interface{} = obj
	for event := range result.ResultChan() {
		switch event.Type {
		case watch.Added:
			fallthrough
		case watch.Modified:
			fallthrough
		case watch.Deleted:
			last = event.Object
			done, err := cb(event.Object)
			if err != nil || done {
				return err
			}
		}
	}

	return fmt.Errorf("timeout waiting condition: %v", last)
}
