package clustermanager

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	clusterController "github.com/rancher/rancher/pkg/controllers/managementuser"
	"github.com/rancher/rancher/pkg/dialer"
	corecontrollers "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/wrangler/v3/pkg/ratelimit"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Manager struct {
	httpsPort     int
	ScaledContext *config.ScaledContext
	clusterLister mgmtcontrollers.ClusterLister
	clusters      mgmtcontrollers.ClusterInterface
	secretLister  corecontrollers.SecretLister
	controllers   sync.Map
	// failures holds, by cluster UID, the *startFailure of the last start of a cluster's controllers, so
	// it can still be reported once the failed record is gone.
	failures      sync.Map
	accessControl types.AccessControl
	startSem      *semaphore.Weighted
}

const (
	// startRetryBaseDelay is how long to wait before starting the controllers of a cluster again after
	// their first failed start. It doubles with every failure that follows, up to startRetryMaxDelay.
	startRetryBaseDelay = 5 * time.Second
	startRetryMaxDelay  = 5 * time.Minute
)

// errDeferredStart is the failure recorded when controllers registered after the start fail to start.
var errDeferredStart = errors.New("deferred controllers failed to start")

// startFailure is a failed start of the controllers of a cluster.
type startFailure struct {
	err error
	// attempts is how many starts in a row have failed.
	attempts int
	// retryAt is when the controllers may be started again. It is zero if they may be right away.
	retryAt time.Time
}

// startRetryDelay returns how long to wait before another start after attempts starts in a row failed.
func startRetryDelay(attempts int) time.Duration {
	delay := startRetryBaseDelay
	for i := 1; i < attempts && delay < startRetryMaxDelay; i++ {
		delay *= 2
	}
	return min(delay, startRetryMaxDelay)
}

// ControllersPhase is where the controllers of a cluster are in their lifecycle on this replica.
type ControllersPhase string

const (
	// ControllersStopped means no controllers are running for the cluster on this replica.
	ControllersStopped ControllersPhase = "Stopped"
	// ControllersStarting means the controllers were asked to start and haven't finished starting yet.
	ControllersStarting ControllersPhase = "Starting"
	// ControllersRunning means the controllers started successfully.
	ControllersRunning ControllersPhase = "Running"
	// ControllersFailed means the last start failed. Its error is in ControllersState.Err.
	ControllersFailed ControllersPhase = "Failed"
)

// ControllersState describes the controllers of a cluster on this replica.
type ControllersState struct {
	Phase ControllersPhase
	// Owner is whether the controllers were started as the cluster's owner.
	Owner bool
	// PreBootstrapped is whether the cluster was pre-bootstrapped by the current start.
	PreBootstrapped bool
	// Err is why the last start failed, if Phase is ControllersFailed.
	Err error
	// RetryAt is when the controllers may be started again after they failed, if Phase is
	// ControllersFailed. It is zero if they may be right away.
	RetryAt time.Time
}

type record struct {
	sync.Mutex
	// clusterRec is the cluster as it was when the record was built. It is the record's own copy, never
	// the informer's: it is handed to the cluster's controllers, and is compared with later versions of
	// the cluster to tell whether the record is out of date. It must not be changed.
	clusterRec      *apimgmtv3.Cluster
	cluster         *config.UserContext
	accessControl   types.AccessControl
	started         bool
	owner           bool
	phase           ControllersPhase
	preBootstrapped bool
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewManager(httpsPort int, context *config.ScaledContext, asl accesscontrol.AccessSetLookup) *Manager {
	return &Manager{
		httpsPort:     httpsPort,
		ScaledContext: context,
		accessControl: rbac.NewAccessControlWithASL("", asl),
		clusterLister: context.Management.Clusters("").Controller().Lister(),
		clusters:      context.Management.Clusters(""),
		secretLister:  context.Core.Secrets("").Controller().Lister(),
		startSem:      semaphore.NewWeighted(int64(settings.ClusterControllerStartCount.GetInt())),
	}
}

// Stop stops the controllers of cluster and forgets why they last failed to start, if they did. A stop
// asked for from outside is deliberate - the cluster is gone, or its controllers are being restarted - so
// an earlier failure no longer describes them, and ControllersState reports them stopped rather than
// failed. Failures noticed by the manager itself go through stopWithError or handleStartFailure instead,
// which record the failure after stopping.
func (m *Manager) Stop(cluster *apimgmtv3.Cluster) {
	m.failures.Delete(cluster.UID)
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return
	}
	m.stopRecord(obj.(*record))
}

// stopWithError stops the controllers of cluster because they couldn't be started with err, and records
// err so ControllersState reports them failed. err has to be recorded after stopping, since Stop forgets
// the failure.
//
// No retry delay is recorded: err is returned to the caller of Start, so the controller loop backs the
// cluster off itself.
func (m *Manager) stopWithError(cluster *apimgmtv3.Cluster, err error) {
	m.Stop(cluster)
	m.failures.Store(cluster.UID, &startFailure{err: err})
}

// ControllersState reports the state of the controllers of cluster on this replica. The state of a
// cluster's current record takes precedence over a failure left by an earlier one.
func (m *Manager) ControllersState(cluster *apimgmtv3.Cluster) ControllersState {
	if obj, ok := m.controllers.Load(cluster.UID); ok {
		r := obj.(*record)
		r.Lock()
		defer r.Unlock()
		if r.started {
			return ControllersState{Phase: r.phase, Owner: r.owner, PreBootstrapped: r.preBootstrapped}
		}
	}
	if obj, ok := m.failures.Load(cluster.UID); ok {
		failure := obj.(*startFailure)
		return ControllersState{Phase: ControllersFailed, Err: failure.err, RetryAt: failure.retryAt}
	}
	return ControllersState{Phase: ControllersStopped}
}

// enqueue asks the cluster controller to sync the cluster named name, so its handlers pick up a change in
// the state of its controllers.
func (m *Manager) enqueue(name string) {
	m.clusters.Controller().Enqueue("", name)
}

func (m *Manager) isActive(r *record) bool {
	obj, ok := m.controllers.Load(r.clusterRec.UID)
	return ok && obj.(*record) == r
}

// stopRecord stops r and removes it from the manager, but only if it is still the active record for
// its cluster. Callbacks held by a record can outlive it - a deferred start reports a failure long
// after it began - and tearing down whichever record happens to be current would stop controllers
// that are working fine. The check and the removal have to be atomic, otherwise a replacement
// installed in between would be the one deleted.
//
// Bailing out leaves nothing behind. The entry for a cluster only changes through this function and
// through the LoadOrStore in start, which only fills an empty one, so a record that is no longer
// the active one was already canceled by whoever removed it.
//
// It reports whether r was the active record and was stopped.
func (m *Manager) stopRecord(r *record) bool {
	if !m.controllers.CompareAndDelete(r.clusterRec.UID, r) {
		return false
	}
	logrus.Infof("Stopping cluster agent for %s", r.cluster.ClusterName)
	r.cancel()
	return true
}

func (m *Manager) Start(ctx context.Context, cluster *apimgmtv3.Cluster, clusterOwner bool) error {
	// reload cluster, always use the cached one
	uid := cluster.UID
	cluster, err := m.clusterLister.Get("", cluster.Name)
	if err != nil {
		return err
	}

	if uid != cluster.UID {
		return errors.New("mismatched cluster UID")
	}

	_, err = m.start(ctx, cluster, true, clusterOwner)
	return err
}

func (m *Manager) RESTConfig(cluster *apimgmtv3.Cluster) (rest.Config, error) {
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return rest.Config{}, fmt.Errorf("cluster record not found %s %s", cluster.Name, cluster.UID)
	}

	record := obj.(*record)
	return record.cluster.RESTConfig, nil
}

// start returns the record for cluster, building it if there is none or if the one there is out of date,
// and starts its controllers if controllers is true.
//
// Only a start with controllers - the user controllers' sync - manages the cluster's controllers. A start
// without them serves a request for a client and leaves running controllers alone: it doesn't replace
// their record, even with out of date connection details, and it doesn't record its failures. Tearing
// down the controllers is up to the sync, which it asks for instead.
func (m *Manager) start(ctx context.Context, cluster *apimgmtv3.Cluster, controllers, clusterOwner bool) (*record, error) {
	if !startAllowed(cluster, controllers) {
		return nil, nil
	}
	obj, ok := m.controllers.Load(cluster.UID)
	if ok {
		r := obj.(*record)
		if !m.changed(r, cluster, controllers, clusterOwner) {
			return r, m.startController(r, controllers, clusterOwner)
		}
		if controllers {
			// Stop the record that was found to be out of date, not whichever is current by now.
			m.stopRecord(r)
		} else if !m.stopIfNotStarted(r) {
			m.enqueue(cluster.Name)
			return r, nil
		}
	}

	clusterRecord, err := m.toRecord(ctx, cluster)
	if err != nil {
		if controllers {
			m.stopWithError(cluster, err)
		}
		return nil, err
	}
	if clusterRecord == nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, "cluster not found")
	}

	obj, _ = m.controllers.LoadOrStore(cluster.UID, clusterRecord)
	if obj.(*record) != clusterRecord {
		// Another goroutine installed a record for this cluster first. The one just built was never
		// started, but it holds a cancel func that would otherwise live as long as the manager.
		clusterRecord.cancel()
	}
	if err := m.startController(obj.(*record), controllers, clusterOwner); err != nil {
		return nil, err
	}

	return obj.(*record), nil
}

// errRecordReplaced is returned when the controllers of a record are to be started after the record was
// removed. The caller is to start again, which finds the record that replaced it.
var errRecordReplaced = errors.New("cluster record was replaced while starting its controllers")

// stopIfNotStarted stops r if its controllers haven't been started, and reports whether they hadn't. The
// check and the stop are done under r's lock, the one startController takes, so r can't have its
// controllers started in between, and startController won't start them once r is stopped.
func (m *Manager) stopIfNotStarted(r *record) bool {
	r.Lock()
	defer r.Unlock()
	if r.started {
		return false
	}
	m.stopRecord(r)
	return true
}

func (m *Manager) startController(r *record, controllers, clusterOwner bool) error {
	if !controllers {
		return nil
	}

	r.Lock()
	defer r.Unlock()
	if !r.started {
		// r may have been stopped by stopIfNotStarted since the caller found it.
		if !m.isActive(r) {
			return errRecordReplaced
		}
		r.started = true
		r.owner = clusterOwner
		r.phase = ControllersStarting
		go func() {
			if err := m.doStart(r, clusterOwner); err != nil {
				m.handleStartFailure(r, err)
			} else {
				m.handleStartSuccess(r)
			}
		}()
	}
	return nil
}

// handleStartSuccess records that the controllers of r are running and asks for the cluster to be synced
// so that is reported. Nothing is recorded if r was stopped or replaced while it was starting.
func (m *Manager) handleStartSuccess(r *record) {
	r.Lock()
	r.phase = ControllersRunning
	r.Unlock()

	if !m.isActive(r) {
		return
	}
	m.failures.Delete(r.clusterRec.UID)
	m.enqueue(r.clusterRec.Name)
}

// startAllowed reports whether a record may be started for cluster. Nothing is started for a cluster being
// removed, except clients without controllers until its agent uninstall has been recorded: until then, its
// removal still cleans up through the downstream cluster, for instance what its role template bindings
// granted there.
func startAllowed(cluster *apimgmtv3.Cluster, controllers bool) bool {
	if cluster.DeletionTimestamp == nil {
		return true
	}
	if controllers {
		return false
	}
	return !apimgmtv3.ClusterConditionAgentUninstallScheduled.IsTrue(cluster) && !apimgmtv3.ClusterConditionAgentUninstallScheduled.IsFalse(cluster)
}

// handleStartFailure stops r after its controllers failed to start with err, records the failure and asks
// for the cluster to be synced so it is reported. It only acts while r is the cluster's active record: r
// may already have been stopped or replaced, for example because the cluster's connection details changed
// while it was starting, and then its failure is not the cluster's, and the replacement must keep running.
//
// The failure carries when the controllers may be started again, later with every failure in a row. The
// sync this enqueues reports the failure straight away but holds off the next start until then.
func (m *Manager) handleStartFailure(r *record, err error) {
	logrus.Errorf("failed to start cluster controllers %s: %v", r.cluster.ClusterName, err)

	r.Lock()
	r.phase = ControllersFailed
	r.Unlock()

	if !m.stopRecord(r) {
		return
	}
	attempts := 1
	if obj, ok := m.failures.Load(r.clusterRec.UID); ok {
		attempts = obj.(*startFailure).attempts + 1
	}
	m.failures.Store(r.clusterRec.UID, &startFailure{
		err:      err,
		attempts: attempts,
		retryAt:  time.Now().Add(startRetryDelay(attempts)),
	})
	m.enqueue(r.clusterRec.Name)
}

func (m *Manager) changed(r *record, cluster *apimgmtv3.Cluster, controllers, clusterOwner bool) bool {
	existing := r.clusterRec
	if existing.Status.APIEndpoint != cluster.Status.APIEndpoint ||
		existing.Status.ServiceAccountTokenSecret != cluster.Status.ServiceAccountTokenSecret ||
		existing.Status.CACert != cluster.Status.CACert ||
		existing.Status.AppliedSpec.LocalClusterAuthEndpoint.Enabled != cluster.Status.AppliedSpec.LocalClusterAuthEndpoint.Enabled {
		return true
	}

	if controllers && r.started && clusterOwner != r.owner {
		return true
	}

	return false
}

func (m *Manager) doStart(rec *record, clusterOwner bool) (exit error) {
	defer func() {
		if exit == nil {
			logrus.Infof("Starting cluster agent for %s [owner=%v]", rec.cluster.ClusterName, clusterOwner)
		}
	}()

	for i := 0; ; i++ {
		// Prior to k8s v1.14, we simply did a DiscoveryClient.Version() check to see if the user cluster is alive
		// As of k8s v1.14, kubeapi returns a successful version response even if etcd is not available.
		// To work around this, now we try to get a namespace from the API, even if not found, it means the API is up.
		if _, err := rec.cluster.K8sClient.CoreV1().Namespaces().Get(rec.ctx, "kube-system", metav1.GetOptions{}); err != nil && !apierrors.IsNotFound(err) {
			if i == 2 {
				if rec.ctx.Err() != nil || errors.Is(err, dialer.ErrAgentDisconnected) {
					return err
				}
				return err
			}
			select {
			case <-rec.ctx.Done():
				return rec.ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}

		break
	}

	if err := m.startSem.Acquire(rec.ctx, 1); err != nil {
		return err
	}
	defer m.startSem.Release(1)

	transaction := controller.NewHandlerTransaction(rec.ctx)

	// pre-bootstrap the cluster if it's not already bootstrapped. A cluster that doesn't need it counts as
	// pre-bootstrapped, so its cluster-agent isn't deployed in pre-bootstrap mode.
	if capr.ShouldPreBootstrap(rec.clusterRec) {
		err := clusterController.PreBootstrap(transaction, m.ScaledContext, rec.cluster, rec.clusterRec)
		if err != nil {
			transaction.Rollback()
			return err
		}
	}
	rec.Lock()
	rec.preBootstrapped = true
	rec.Unlock()
	// Report it now rather than once the controllers are running, which can take a while: the
	// cluster-agent is deployed in pre-bootstrap mode until the cluster is marked.
	m.enqueue(rec.clusterRec.Name)

	if clusterOwner {
		if err := clusterController.Register(transaction, m.ScaledContext, rec.cluster, rec.clusterRec, m); err != nil {
			transaction.Rollback()
			return err
		}
	} else {
		if err := clusterController.RegisterFollower(rec.cluster); err != nil {
			transaction.Rollback()
			return err
		}
	}

	done := make(chan error, 1)
	go func() {
		defer close(done)

		logrus.Debugf("[clustermanager] creating AccessControl for cluster %v", rec.cluster.ClusterName)
		rec.accessControl = rbac.NewAccessControl(transaction, rec.cluster.ClusterName, rec.cluster.RBACw)

		err := rec.cluster.Start(rec.ctx)
		if err == nil {
			transaction.Commit()
		} else {
			transaction.Rollback()
		}
		done <- err
	}()

	select {
	case <-time.After(10 * time.Minute):
		rec.cancel()
		return errors.New("timed out waiting for cluster controllers to start")
	case err := <-done:
		return err
	}
}

// ToRESTConfig generates a rest.Config for a given cluster.
// If reconnect is true, the dialer used for connecting this rest.Config will block
// and retry connecting to the cluster for ~30s if the connection is not available,
// otherwise return immediately
func ToRESTConfig(cluster *apimgmtv3.Cluster, context *config.ScaledContext, secretLister corecontrollers.SecretLister, reconnect bool) (*rest.Config, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.Spec.Internal {
		return &context.RESTConfig, nil
	}

	if cluster.Status.APIEndpoint == "" || cluster.Status.CACert == "" {
		return nil, nil
	}

	if !apimgmtv3.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil, nil
	}

	// clusters that don't have a service account token secret can't interact with downstream yet
	if !apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.IsTrue(cluster) {
		return nil, fmt.Errorf("waiting for service account token secret to be populated for cluster [%s]", cluster.Name)
	}

	u, err := url.Parse(cluster.Status.APIEndpoint)
	if err != nil {
		return nil, err
	}

	caBytes, err := base64.StdEncoding.DecodeString(cluster.Status.CACert)
	if err != nil {
		return nil, err
	}

	clusterDialer, err := context.Dialer.ClusterDialer(cluster.Name, reconnect)
	if err != nil {
		return nil, err
	}

	secret, err := secretLister.Get(secretmigrator.SecretNamespace, cluster.Status.ServiceAccountTokenSecret)
	if err != nil {
		return nil, err
	}

	// adding suffix to make tlsConfig hashkey unique
	suffix := []byte("\n" + cluster.Name)
	rc := &rest.Config{
		Host:        u.String(),
		Dial:        clusterDialer,
		BearerToken: string(secret.Data[secretmigrator.SecretKey]),
		CAData:      append(caBytes, suffix...),
		NextProtos:  []string{"http/1.1"},
		Timeout:     45 * time.Second,
		RateLimiter: ratelimit.None,
		UserAgent:   rest.DefaultKubernetesUserAgent() + " cluster " + cluster.Name,
	}

	return rc, nil
}

func (m *Manager) toRecord(ctx context.Context, cluster *apimgmtv3.Cluster) (*record, error) {
	kubeConfig, err := ToRESTConfig(cluster, m.ScaledContext, m.secretLister, true)
	if kubeConfig == nil || err != nil {
		return nil, err
	}

	clusterContext, err := config.NewUserContext(m.ScaledContext, *kubeConfig, cluster.Name)
	if err != nil {
		return nil, err
	}
	clusterContext.ClusterUID = cluster.UID

	return m.newRecord(ctx, cluster, clusterContext), nil
}

// newRecord returns a record for cluster that serves clusterContext. cluster is usually the informer's
// copy, so the record keeps a copy of its own.
func (m *Manager) newRecord(ctx context.Context, cluster *apimgmtv3.Cluster, clusterContext *config.UserContext) *record {
	s := &record{
		cluster:    clusterContext,
		clusterRec: cluster.DeepCopy(),
	}
	s.ctx, s.cancel = context.WithCancel(ctx)

	clusterContext.OnDeferredStartError = func() {
		// Dropping the record is what makes the next start build a new UserContext instead of finding
		// this one and leaving it alone. Like any failed start, it is retried after a delay.
		m.handleStartFailure(s, errDeferredStart)
	}

	return s
}

func (m *Manager) AccessControl(apiContext *types.APIContext, storageContext types.StorageContext) (types.AccessControl, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.accessControl, nil
	}

	if record.accessControl == nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, "cannot determine access, cluster is unavailable")
	}

	return record.accessControl, nil
}

func (m *Manager) UnversionedClient(apiContext *types.APIContext, storageContext types.StorageContext) (rest.Interface, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.ScaledContext.UnversionedClient, nil
	}
	return record.cluster.UnversionedClient, nil
}

func (m *Manager) APIExtClient(_ *types.APIContext, _ types.StorageContext) (clientset.Interface, error) {
	return m.ScaledContext.APIExtClient, nil
}

// UserContextNoControllers accepts a cluster name and returns a client for that cluster,
// no controllers are started for that cluster in the process.
// Note it will block retrying to connect to the cluster for ~30 seconds before returning
// in case the cluster connection fails.
func (m *Manager) UserContextNoControllers(clusterName string) (*config.UserContext, error) {
	return m.UserContextNoControllersReconnecting(clusterName, true)
}

type ContextError struct {
	Message string
}

func (e *ContextError) Error() string {
	return e.Message
}

// UserContextNoControllersReconnecting works like UserContextNoControllers if reconnect is true.
// Otherwise, it will return an error immediately if the cluster connection fails.
func (m *Manager) UserContextNoControllersReconnecting(clusterName string, reconnect bool) (*config.UserContext, error) {
	cluster, err := m.clusterLister.Get("", clusterName)
	if err != nil {
		return nil, err
	}
	ctx, err := m.UserContextFromClusterReconnecting(cluster, reconnect)
	if ctx == nil && err == nil {
		return nil, &ContextError{
			Message: fmt.Sprintf("cluster context %s is unavailable", clusterName),
		}
	}
	return ctx, err
}

// UserContext accepts a cluster name and returns a client for that cluster,
// starting all controllers for that cluster in the process.
func (m *Manager) UserContext(clusterName string) (*config.UserContext, error) {
	cluster, err := m.clusterLister.Get("", clusterName)
	if err != nil {
		return nil, err
	}

	record, err := m.start(context.Background(), cluster, false, false)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, err.Error())
	}

	if record == nil {
		return nil, httperror.NewAPIError(httperror.NotFound, "failed to find cluster")
	}

	return record.cluster, nil
}

// UserContextFromCluster accepts a pointer to a Cluster and returns a client
// for that cluster. It does not start any controllers.
// Note it will block retrying to connect to the cluster for ~30 seconds before returning
// in case the cluster connection fails.
func (m *Manager) UserContextFromCluster(cluster *apimgmtv3.Cluster) (*config.UserContext, error) {
	return m.UserContextFromClusterReconnecting(cluster, true)
}

// UserContextFromClusterReconnecting works like UserContextFromCluster if reconnect is true.
// Otherwise, it will return an error immediately if the cluster connection fails.
func (m *Manager) UserContextFromClusterReconnecting(cluster *apimgmtv3.Cluster, reconnect bool) (*config.UserContext, error) {
	kubeConfig, err := ToRESTConfig(cluster, m.ScaledContext, m.secretLister, reconnect)
	if err != nil {
		return nil, err
	}
	if kubeConfig == nil {
		logrus.Debugf("could not get kubeconfig for cluster %s", cluster.Name)
		return nil, nil
	}
	clusterContext, err := config.NewUserContext(m.ScaledContext, *kubeConfig, cluster.Name)
	if err != nil {
		return nil, err
	}
	clusterContext.ClusterUID = cluster.UID
	return clusterContext, nil
}

func (m *Manager) record(apiContext *types.APIContext, storageContext types.StorageContext) (*record, error) {
	if apiContext == nil {
		return nil, nil
	}
	cluster, err := m.cluster(apiContext, storageContext)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, err.Error())
	}
	if cluster == nil {
		return nil, nil
	}
	record, err := m.start(context.Background(), cluster, false, false)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, err.Error())
	}

	return record, nil
}

func (m *Manager) ClusterName(apiContext *types.APIContext) string {
	clusterID := apiContext.SubContext["/v3/schemas/cluster"]
	if clusterID == "" {
		projectID, ok := apiContext.SubContext["/v3/schemas/project"]
		if ok {
			parts := strings.SplitN(projectID, ":", 2)
			if len(parts) == 2 {
				clusterID = parts[0]
			}
		}
	}
	return clusterID
}

func (m *Manager) cluster(apiContext *types.APIContext, context types.StorageContext) (*apimgmtv3.Cluster, error) {
	switch context {
	case types.DefaultStorageContext:
		return nil, nil
	case config.ManagementStorageContext:
		return nil, nil
	case config.UserStorageContext:
	default:
		return nil, fmt.Errorf("illegal context: %s", context)

	}

	clusterID := m.ClusterName(apiContext)
	if clusterID == "" {
		return nil, nil
	}

	return m.clusterLister.Get("", clusterID)
}

func (m *Manager) KubeConfig(clusterName, token string) *clientcmdapi.Config {
	return &clientcmdapi.Config{
		CurrentContext: "default",
		APIVersion:     "v1",
		Kind:           "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			"default": {
				Server:                fmt.Sprintf("https://localhost:%d/k8s/clusters/%s", m.httpsPort, clusterName),
				InsecureSkipTLSVerify: true,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				AuthInfo: "user",
				Cluster:  "default",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				Token: token,
			},
		},
	}
}

func (m *Manager) GetHTTPSPort() int {
	return m.httpsPort
}

func (m *Manager) SubjectAccessReviewForCluster(req *http.Request) (authv1.SubjectAccessReviewInterface, error) {
	clusterID := clusterrouter.GetClusterID(req)
	userContext, err := m.UserContextNoControllers(clusterID)
	if err != nil {
		return nil, err
	}
	return userContext.K8sClient.AuthorizationV1().SubjectAccessReviews(), nil
}
