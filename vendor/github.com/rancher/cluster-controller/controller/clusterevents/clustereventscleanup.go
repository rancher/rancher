package clusterevents

import (
	"context"
	"time"

	"github.com/rancher/cluster-agent/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	syncInterval = 1 * time.Hour
	TTL          = 24 * time.Hour
)

type Cleaner struct {
	clusterEventsClient v3.ClusterEventInterface
	clusterEvents       v3.ClusterEventLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := &Cleaner{
		clusterEventsClient: management.Management.ClusterEvents(""),
		clusterEvents:       management.Management.ClusterEvents("").Controller().Lister(),
	}
	go c.sync(ctx, syncInterval)
}

func (c *Cleaner) sync(ctx context.Context, syncInterval time.Duration) {
	for range utils.TickerContext(ctx, syncInterval) {
		err := c.cleanup()
		if err != nil {
			logrus.Errorf("Error running cluster events cleanup thread %v", err)
		}
	}
}

func (c *Cleaner) cleanup() error {
	logrus.Infof("Running cluster events cleanup")
	events, err := c.clusterEvents.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	for _, event := range events {
		created := event.CreationTimestamp.Time
		if time.Now().Sub(created) >= TTL {
			logrus.Debugf("Cleaninig up cluster event %s", event.Message)
			err := c.clusterEventsClient.Delete(event.Name, &metav1.DeleteOptions{})
			if err != nil {
				// just log the error, retry will happen as a part of the next run
				logrus.Errorf("Error deleting cluster event %s: %v", event.Message, err)
			}
		}
	}
	logrus.Infof("Done running cluster events cleanup")

	return nil
}
