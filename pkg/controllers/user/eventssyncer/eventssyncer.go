package eventssyncer

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	projectIDLabel = "field.cattle.io/projectId"
)

type EventsSyncer struct {
	clusterName          string
	clusters             v3.ClusterLister
	clusterEvents        v3.ClusterEventLister
	clusterEventsClient  v3.ClusterEventInterface
	clusterNamespaces    v1.NamespaceLister
	managementNamespaces v1.NamespaceLister
}

func Register(workload *config.UserContext) {
	e := &EventsSyncer{
		clusterName:          workload.ClusterName,
		clusters:             workload.Management.Management.Clusters("").Controller().Lister(),
		clusterEventsClient:  workload.Management.Management.ClusterEvents(""),
		clusterNamespaces:    workload.Core.Namespaces("").Controller().Lister(),
		managementNamespaces: workload.Management.Core.Namespaces("").Controller().Lister(),
		clusterEvents:        workload.Management.Management.ClusterEvents("").Controller().Lister(),
	}
	workload.Core.Events("").Controller().AddHandler("events-syncer", e.sync)
}

func (e *EventsSyncer) sync(key string, event *corev1.Event) error {
	if event == nil {
		return nil
	}
	return e.createClusterEvent(key, event)
}

func (e *EventsSyncer) createClusterEvent(key string, event *corev1.Event) error {
	ns, err := e.getEventNamespaceName(event)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Warnf("Error propagating event [%s]: %v", event.Message, err)
			return nil
		}
		return err
	}
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil
	}
	existing, err := e.clusterEvents.Get(ns.Name, event.Name)
	if err == nil || apierrors.IsNotFound(err) {
		if existing != nil && existing.Name != "" {
			return nil
		}
		cluster, err := e.clusters.Get("", e.clusterName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return errors.Wrapf(err, "Failed to get cluster [%s]", e.clusterName)
		}
		if cluster.DeletionTimestamp != nil {
			return nil
		}

		logrus.Debugf("Creating cluster event [%s]", event.Message)
		clusterEvent := e.convertEventToClusterEvent(event, ns)
		_, err = e.clusterEventsClient.Create(clusterEvent)
		return err
	}

	return err
}

func (e *EventsSyncer) convertEventToClusterEvent(event *corev1.Event, ns *corev1.Namespace) *v3.ClusterEvent {
	clusterEvent := &v3.ClusterEvent{
		Event: *event,
	}
	clusterEvent.APIVersion = "management.cattle.io/v3"
	clusterEvent.Kind = "ClusterEvent"
	clusterEvent.ClusterName = e.clusterName
	clusterEvent.ObjectMeta = metav1.ObjectMeta{
		Name:        event.Name,
		Labels:      event.Labels,
		Annotations: event.Annotations,
		Namespace:   ns.Name,
	}
	return clusterEvent
}

func (e *EventsSyncer) getEventNamespaceName(event *corev1.Event) (*corev1.Namespace, error) {
	involedObjectNamespace := event.InvolvedObject.Namespace
	if involedObjectNamespace == "" {
		// cluster namespace, equals to cluster.name
		namespace, err := e.managementNamespaces.Get("", e.clusterName)
		if err != nil {
			return nil, err
		}
		return namespace, nil
	}

	// user namespace, derive from the project id
	// field.cattle.io/projectId value is <cluster name>:<project name>
	userNamespace, err := e.clusterNamespaces.Get("", involedObjectNamespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to find user namespace [%s]", e.clusterName)
	}
	if userNamespace.Annotations[projectIDLabel] != "" {
		parts := strings.Split(userNamespace.Annotations[projectIDLabel], ":")
		if len(parts) == 2 {
			// project namespace name == project name
			projectNamespaceName := parts[1]
			namespace, err := e.managementNamespaces.Get("", projectNamespaceName)
			if err != nil {
				return nil, err
			}
			return namespace, nil
		}
	}
	return nil, nil
}
