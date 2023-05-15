package wait

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type WatchFunc func(namespace string, opts metav1.ListOptions) (watch.Interface, error)
type WatchClusterScopedFunc func(opts metav1.ListOptions) (watch.Interface, error)
type watchFunc func() (watch.Interface, error)

func ClusterScopedList(ctx context.Context, watchFunc WatchClusterScopedFunc, cb func(obj runtime.Object) (bool, error)) error {
	return retryWatch(ctx, func() (watch.Interface, error) {
		return watchFunc(metav1.ListOptions{})
	}, cb)
}

func doWatch(ctx context.Context, watchFunc watchFunc, cb func(obj runtime.Object) (bool, error)) (bool, error) {
	result, err := watchFunc()
	if err != nil {
		logrus.Error("watch failed", err)
		time.Sleep(2 * time.Second)
		return false, nil
	}
	defer func() {
		result.Stop()
		for range result.ResultChan() {
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting condition: %w", ctx.Err())
		case event, open := <-result.ResultChan():
			if !open {
				return false, nil
			}
			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				done, err := cb(event.Object)
				if err != nil || done {
					if apierrors.IsConflict(err) {
						// if we got a conflict, return a false (not done) and nil for error
						return false, nil
					}
					return true, err
				}
			}
		}
	}
}

func retryWatch(ctx context.Context, watchFunc watchFunc, cb func(obj runtime.Object) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(defaults.WatchTimeoutSeconds)*time.Second)
	defer cancel()
	for {
		if done, err := doWatch(ctx, watchFunc, cb); err != nil || done {
			return err
		}
	}
}

func Object(ctx context.Context, watchFunc WatchFunc, obj runtime.Object, cb func(obj runtime.Object) (bool, error)) error {
	if done, err := cb(obj); err != nil || done {
		return err
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	return retryWatch(ctx, func() (watch.Interface, error) {
		return watchFunc(meta.GetNamespace(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + meta.GetName(),
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
	}, cb)
}

func EnsureDoesNotExist(ctx context.Context, getter func() (runtime.Object, error)) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(defaults.WatchTimeoutSeconds)*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deletion: %w", ctx.Err())
		case <-ticker.C:
			_, err := getter()
			if apierrors.IsNotFound(err) {
				return nil
			} else if err != nil {
				return err
			}
		}
	}
}
