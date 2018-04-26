package leader

import (
	"context"
	"os"

	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"
)

type LeaderState struct {
	sync.Mutex
	identity string
	leader   bool
}

func (l *LeaderState) Get() (string, bool) {
	l.Lock()
	defer l.Unlock()
	return l.identity, l.leader
}

func (l *LeaderState) Status(identity string, leader bool) {
	l.Lock()
	l.identity = identity
	l.leader = leader
	l.Unlock()
}

type Callback func(cb context.Context)
type StatusCallback func(identity string, leader bool)

func RunOrDie(ctx context.Context, name string, client kubernetes.Interface, cb Callback, status StatusCallback) {
	err := run(ctx, name, client, cb, status)
	if err != nil {
		logrus.Fatalf("Failed to start leader election for %s", name)
	}
	panic("Failed to start leader election for " + name)
}

func run(ctx context.Context, name string, client kubernetes.Interface, cb Callback, status StatusCallback) error {
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	le := leaderelectionconfig.DefaultLeaderElectionConfiguration()
	le.LeaderElect = true
	le.ResourceLock = resourcelock.ConfigMapsResourceLock

	recorder := createRecorder(name, client)

	rl, err := resourcelock.New(le.ResourceLock,
		"kube-system",
		name,
		client.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		})
	if err != nil {
		logrus.Fatalf("error creating leader lock for %s: %v", name, err)
	}

	status(id, false)

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: le.LeaseDuration.Duration,
		RenewDeadline: le.RenewDeadline.Duration,
		RetryPeriod:   le.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				subCtx, cancel := context.WithCancel(ctx)
				go cb(subCtx)
				status(id, true)
				<-stop
				cancel()
			},
			OnStoppedLeading: func() {
				logrus.Fatalf("leaderelection lost for %s", name)
			},
			OnNewLeader: func(identity string) {
				status(identity, identity == id)
			},
		},
	})
	panic("unreachable")
}

func createRecorder(name string, kubeClient kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(api.Scheme, v1.EventSource{Component: name})
}
