package clusterprovisioner

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/rancher/kontainer-engine/logstream"
	"github.com/rancher/norman/condition"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provisioner) logEvent(cluster *v3.Cluster, event logstream.LogEvent, cond condition.Cond) *v3.Cluster {
	if event.Error {
		p.EventLogger.Error(cluster, event.Message)
		logrus.Errorf("cluster [%s] provisioning: %s", cluster.Name, event.Message)
	} else {
		p.EventLogger.Info(cluster, event.Message)
		logrus.Infof("cluster [%s] provisioning: %s", cluster.Name, event.Message)
	}
	if cond.GetMessage(cluster) != event.Message {
		updated := false
		for i := 0; i < 2 && !updated; i++ {
			if event.Error {
				cond.False(cluster)
			}
			cond.Message(cluster, event.Message)
			if newCluster, err := p.Clusters.Update(cluster); err == nil {
				updated = true
				cluster = newCluster
			} else {
				newCluster, err = p.Clusters.Get(cluster.Name, metav1.GetOptions{})
				if err == nil {
					cluster = newCluster
				}
			}
		}
	}
	return cluster
}

func (p *Provisioner) getCtx(cluster *v3.Cluster, cond condition.Cond) (context.Context, io.Closer) {
	logger := logstream.NewLogStream()
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{
		"log-id": logger.ID(),
	}))
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for event := range logger.Stream() {
			cluster = p.logEvent(cluster, event, cond)
		}
	}()

	return ctx, closerFunc(func() error {
		logger.Close()
		wg.Wait()
		return nil
	})
}

func (p *Provisioner) recordFailure(cluster *v3.Cluster, spec v3.ClusterSpec, err error) *v3.Cluster {
	if err == nil {
		p.backoff.DeleteEntry(cluster.Name)
		if cluster.Status.FailedSpec == nil {
			return cluster
		}

		cluster.Status.FailedSpec = nil
		newCluster, err := p.Clusters.Update(cluster)
		if err == nil {
			return newCluster
		}
		// mask the error
		return cluster
	}

	p.backoff.Next(cluster.Name, time.Now())
	cluster.Status.FailedSpec = &spec
	newCluster, err := p.Clusters.Update(cluster)
	if err == nil {
		return newCluster
	}

	// mask the error
	return cluster
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
