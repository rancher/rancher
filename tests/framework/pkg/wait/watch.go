package wait

import (
	"fmt"

	"k8s.io/apimachinery/pkg/watch"
)

// WatchCheckFunc is the function type of `check` needed for WatchWait e.g.
//
//	 checkFunc := func(event watch.Event) (ready bool, err error) {
//			cluster := event.Object.(*apisV1.Cluster)
//			ready = cluster.Status.Ready
//			return ready, nil
//	 }
type WatchCheckFunc func(watch.Event) (bool, error)

// WatchWait uses the `watchInterface`  to wait until the `check` function to returns true.
// e.g. WatchWait for provisioning a cluster
//
//	 result, err := r.client.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
//			FieldSelector:  "metadata.name=" + clusterName,
//			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
//	 })
//	 require.NoError(r.T(), err)
//	 err = wait.WatchWait(result, checkFunc)
func WatchWait(watchInterface watch.Interface, check WatchCheckFunc) error {
	defer func() {
		watchInterface.Stop()
	}()

	for {
		select {
		case event, open := <-watchInterface.ResultChan():
			if !open {
				return fmt.Errorf("timeout waiting on condition")
			}
			if event.Type == watch.Error {
				return fmt.Errorf("error with watch connection")
			}

			done, err := check(event)
			if err != nil {
				return err
			}

			if done {
				return nil
			}
		}
	}
}
