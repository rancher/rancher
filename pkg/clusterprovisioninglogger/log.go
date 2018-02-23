package clusterprovisioninglogger

import (
	"context"
	"io"
	"sync"

	"github.com/rancher/kontainer-engine/logstream"
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/event"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type logger struct {
	EventLogger event.Logger
	Clusters    v3.ClusterInterface
}

func NewLogger(clusters v3.ClusterInterface, eventLogger event.Logger, cluster *v3.Cluster, cond condition.Cond) (context.Context, io.Closer) {
	l := &logger{
		EventLogger: eventLogger,
		Clusters:    clusters,
	}

	_, ctx, logger := l.getCtx(cluster, cond)
	return ctx, logger
}

func NewNonRPCLogger(clusters v3.ClusterInterface, eventLogger event.Logger, cluster *v3.Cluster, cond condition.Cond) (context.Context, io.Closer) {
	l := &logger{
		EventLogger: eventLogger,
		Clusters:    clusters,
	}

	logID, ctx, logger := l.getCtx(cluster, cond)
	targetLogger := logstream.GetLogStream(logID)
	return log.SetLogger(ctx, targetLogger), logger
}

func (p *logger) logEvent(cluster *v3.Cluster, event logstream.LogEvent, cond condition.Cond) *v3.Cluster {
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

func (p *logger) getCtx(cluster *v3.Cluster, cond condition.Cond) (string, context.Context, io.Closer) {
	logger := logstream.NewLogStream()
	logID := logger.ID()
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{
		"log-id": logID,
	}))
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for event := range logger.Stream() {
			cluster = p.logEvent(cluster, event, cond)
		}
	}()

	return logID, ctx, closerFunc(func() error {
		logger.Close()
		wg.Wait()
		return nil
	})
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
