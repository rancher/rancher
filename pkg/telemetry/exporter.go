package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rancher/rancher/pkg/telemetry/initcond"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type TelemetryExporterManager interface {
	// Registers an exporter. Must be registered before Start
	Register(name string, exp TelemetryExporter, retry time.Duration)
	// Start starts the collection and export background tasks
	Start(ctx context.Context, info initcond.InitInfo) error
	Stop() error
}

func NewTelemetryExporterManager(telG TelemetryGatherer) TelemetryExporterManager {
	started := &atomic.Uint32{}
	started.Store(0)
	return &simpleManager{
		telG:       telG,
		exporterMu: &sync.Mutex{},
		exporters:  map[string]exporterRetry{},
		done:       make(chan struct{}),
		log: logrus.WithFields(
			logrus.Fields{
				"component": "telemetry-broker",
			},
		),
		started: started,
	}
}

type exporterRetry struct {
	exp      TelemetryExporter
	retryDur time.Duration
}

type simpleManager struct {
	exporterMu *sync.Mutex
	exporters  map[string]exporterRetry

	started *atomic.Uint32

	telG TelemetryGatherer
	done chan struct{}
	log  *logrus.Entry
}

func (s *simpleManager) Register(name string, exp TelemetryExporter, retry time.Duration) {
	s.exporterMu.Lock()
	defer s.exporterMu.Unlock()
	if s.started.Load() == 1 {
		return
	}
	s.exporters[name] = exporterRetry{
		exp:      exp,
		retryDur: retry,
	}
}

func (s *simpleManager) Start(ctx context.Context, info initcond.InitInfo) error {
	s.telG.visitWithInitInfo(info)
	s.log.WithField(
		"count", len(s.exporters),
	).Info("starting telemetry gathering")

	if !s.started.CompareAndSwap(0, 1) {
		return fmt.Errorf("already started")
	}

	for name, exporter := range s.exporters {
		exporter := exporter
		exporter.exp.Register(s.telG)
		go func() {
			log := s.log.WithField("telemetry-exporter", name)
			t := time.NewTicker(exporter.retryDur)
			defer t.Stop()

			for {
				select {
				case <-t.C:
					log.Trace("gathering telemetry...")
					if err := exporter.exp.CollectAndExport(); err != nil {
						log.WithError(err).Error("failed to collect and export telemetry data")
					}
					log.Trace("gathered telemetry")
				case <-s.done:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	return nil
}

func (s *simpleManager) Stop() error {
	close(s.done)
	return nil
}

type TelemetryExporter interface {
	Register(telG TelemetryGatherer)
	CollectAndExport() error
}

func NewSecretExporter(secretCtrl wcorev1.SecretController, ref *corev1.SecretReference) *secretTelemetryExporter {
	return &secretTelemetryExporter{
		ctrl:      secretCtrl,
		secretRef: ref,
	}
}

type secretTelemetryExporter struct {
	telG      TelemetryGatherer
	secretRef *corev1.SecretReference
	ctrl      wcorev1.SecretController
}

func (s *secretTelemetryExporter) Register(telG TelemetryGatherer) {
	s.telG = telG
}

func (s *secretTelemetryExporter) CollectAndExport() error {
	telG, err := s.telG.GetClusterTelemetry()
	if err != nil {
		return err
	}
	payload, err := GenerateSCCPayload(telG)
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := s.createOrUpdate(data); err != nil {
		return err
	}

	return nil
}

func (s *secretTelemetryExporter) createOrUpdate(data []byte) error {
	t := time.Now()
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.secretRef.Name,
			Namespace: s.secretRef.Namespace,
			Annotations: map[string]string{
				"scc.cattle.io/part-of":      "telemetry",
				"scc.cattle.io/last-changed": t.Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{
			"payload": data,
		},
	}

	_, err := s.ctrl.Create(sec)

	if err == nil {
		return nil
	}

	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := s.ctrl.Update(sec)
		return err
	}); err != nil {
		return err
	}

	return nil
}
