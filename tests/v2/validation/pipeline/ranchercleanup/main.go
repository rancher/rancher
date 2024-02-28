package main

import (
	"context"
	"fmt"
	"time"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/sirupsen/logrus"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	timeout = int64(60 * 10)
)

func main() {
	testSession := session.NewSession()

	client, err := rancher.NewClient("", testSession)
	if err != nil {
		logrus.Errorf("error creating admin client: %v", err)
	} else {
		var clusterList *v1.SteveCollection
		err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
			//clean up clusters
			resp, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).List(nil)
			if k8sErrors.IsInternalError(err) || k8sErrors.IsServiceUnavailable(err) {
				return false, err
			} else if resp != nil {
				clusterList = resp
				return true, nil
			}
			return false, nil
		})

		if err != nil {
			logrus.Errorf("error retrieving cluster list: %v", err)
		}

		deleteTimeout := timeout
		for _, cluster := range clusterList.Data {
			if cluster.ObjectMeta.Name != "local" {
				err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Delete(&cluster)
				if err != nil {
					logrus.Errorf("error deleting cluster: %v", err)
				}

				provKubeClient, err := client.GetKubeAPIProvisioningClient()
				if err != nil {
					logrus.Errorf("error deleting corral: %v", err)
				}

				watchInterface, err := provKubeClient.Clusters(cluster.ObjectMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{
					FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
					TimeoutSeconds: &deleteTimeout,
				})
				if err != nil {
					logrus.Errorf("error initiating watchInterface: %v", err)
				}

				err = wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
					cluster := event.Object.(*apisV1.Cluster)
					if event.Type == watch.Error {
						return false, fmt.Errorf("there was an error deleting cluster")
					} else if event.Type == watch.Deleted {
						return true, nil
					} else if cluster == nil {
						return true, nil
					}
					return false, nil
				})
				if err != nil {
					logrus.Errorf("error while deleting clusters: %v", err)
				}
			}
		}
	}
	corral.DeleteAllCorrals()
	if err != nil {
		logrus.Errorf("error deleting corrals: %v", err)
	}
}
