package eventssyncer

import (
	"fmt"

	clusterv1 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EventsSyncer struct {
	clusterName   string
	Clusters      clusterv1.ClusterInterface
	ClusterEvents clusterv1.ClusterEventInterface
}

func Register(workload *config.ClusterContext) {
	e := &EventsSyncer{
		clusterName:   workload.ClusterName,
		Clusters:      workload.Management.Management.Clusters(""),
		ClusterEvents: workload.Management.Management.ClusterEvents(""),
	}
	workload.Core.Events("").Controller().AddHandler(e.sync)
}

func (e *EventsSyncer) sync(key string, event *v1.Event) error {
	if event == nil {
		return nil
	}
	return e.createClusterEvent(key, event)
}

func (e *EventsSyncer) createClusterEvent(key string, event *v1.Event) error {
	existing, err := e.ClusterEvents.Get(event.Name, metav1.GetOptions{})

	if err == nil || apierrors.IsNotFound(err) {
		if existing != nil && existing.Name != "" {
			return nil
		}
		cluster, err := e.Clusters.Get(e.clusterName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("Failed to get cluster [%s] %v", e.clusterName, err)
		}

		if cluster.DeletionTimestamp != nil {
			return nil
		}
		logrus.Infof("Creating cluster event [%s]", event.Message)
		clusterEvent := e.convertEventToClusterEvent(event, cluster)
		_, err = e.ClusterEvents.Create(clusterEvent)
		return err
	}

	return err
}

func (e *EventsSyncer) convertEventToClusterEvent(event *v1.Event, cluster *clusterv1.Cluster) *clusterv1.ClusterEvent {
	clusterEvent := &clusterv1.ClusterEvent{
		Event: *event,
	}
	clusterEvent.APIVersion = "management.cattle.io/v3"
	clusterEvent.Kind = "ClusterEvent"
	clusterEvent.ClusterName = e.clusterName
	clusterEvent.ObjectMeta = metav1.ObjectMeta{
		Name:        event.Name,
		Labels:      event.Labels,
		Annotations: event.Annotations,
	}
	ref := metav1.OwnerReference{
		Name:       e.clusterName,
		UID:        cluster.UID,
		APIVersion: cluster.APIVersion,
		Kind:       cluster.Kind,
	}
	clusterEvent.ObjectMeta.OwnerReferences = append(clusterEvent.ObjectMeta.OwnerReferences, ref)
	return clusterEvent
}
