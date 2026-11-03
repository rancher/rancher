// Package authconfigtest provides test helpers for code that reads AuthConfigs.
package authconfigtest

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/util"
	normanv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
)

const authConfigsPath = "/apis/management.cattle.io/v3/authconfigs"

// NewClient returns an AuthConfig client backed by a fake API server that
// serves Get and List for the given AuthConfigs. Each config must have
// metadata.name set.
//
// The client is a real norman client, so both the typed client and
// ObjectClient().UnstructuredClient() work against it.
func NewClient(t *testing.T, configs ...map[string]any) normanv3.AuthConfigInterface {
	t.Helper()

	client, err := normanv3.NewForConfig(rest.Config{Host: newServer(t, configs...)})
	require.NoError(t, err)

	return client.AuthConfigs("")
}

// NewScaledContext returns a ScaledContext for calling a provider's Configure.
// Its management clients are backed by the same fake API server as NewClient,
// which serves only AuthConfigs. Its informers are never started, so listers
// are empty.
func NewScaledContext(t *testing.T, configs ...map[string]any) *config.ScaledContext {
	t.Helper()

	restConfig := rest.Config{Host: newServer(t, configs...)}
	management, err := normanv3.NewForConfig(restConfig)
	require.NoError(t, err)
	coreFactory, err := core.NewFactoryFromConfig(&restConfig)
	require.NoError(t, err)

	return &config.ScaledContext{
		Management: management,
		Wrangler:   &wrangler.Context{Core: coreFactory.Core().V1()},
	}
}

// newServer starts a fake API server that serves Get and List for the given
// AuthConfigs and returns its URL.
func newServer(t *testing.T, configs ...map[string]any) string {
	t.Helper()

	byName := map[string]map[string]any{}
	items := make([]map[string]any, 0, len(configs))
	for _, cfg := range configs {
		cfg = withTypeMeta(cfg)
		name, _ := cfg["metadata"].(map[string]any)["name"].(string)
		require.NotEmpty(t, name, "AuthConfig fixture must have metadata.name")
		byName[name] = cfg
		items = append(items, cfg)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected AuthConfig API request: %s %s", r.Method, r.URL.Path)
			writeStatus(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "")
			return
		}

		switch {
		case r.URL.Path == authConfigsPath:
			if r.URL.Query().Get("watch") == "true" {
				writeStatus(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfigList",
				"metadata":   map[string]any{"resourceVersion": "1"},
				"items":      items,
			})
		case strings.HasPrefix(r.URL.Path, authConfigsPath+"/"):
			name := strings.TrimPrefix(r.URL.Path, authConfigsPath+"/")
			cfg, ok := byName[name]
			if !ok {
				writeStatus(w, http.StatusNotFound, "NotFound", name)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		default:
			writeStatus(w, http.StatusNotFound, "NotFound", "")
		}
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

// NewRecordingClient returns an AuthConfig client backed by a fake API server
// that serves a Get for any name with an enabled AuthConfig of that name and
// no other fields. The returned func reports the names of the AuthConfigs
// fetched, in order.
func NewRecordingClient(t *testing.T) (normanv3.AuthConfigInterface, func() []string) {
	t.Helper()

	var (
		mu      sync.Mutex
		fetched []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, authConfigsPath+"/") {
			t.Errorf("unexpected AuthConfig API request: %s %s", r.Method, r.URL.Path)
			writeStatus(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "")
			return
		}

		name := strings.TrimPrefix(r.URL.Path, authConfigsPath+"/")
		mu.Lock()
		fetched = append(fetched, name)
		mu.Unlock()
		writeJSON(w, http.StatusOK, withTypeMeta(map[string]any{
			"metadata": map[string]any{"name": name},
			"enabled":  true,
		}))
	}))
	t.Cleanup(srv.Close)

	client, err := normanv3.NewForConfig(rest.Config{Host: srv.URL})
	require.NoError(t, err)

	return client.AuthConfigs(""), func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(fetched)
	}
}

// NewLister returns an AuthConfig lister that serves Get and List for the
// given AuthConfigs, which take the same form as for NewClient. Get returns a
// NotFound error for any other name.
func NewLister(t *testing.T, configs ...map[string]any) normanv3.AuthConfigLister {
	t.Helper()

	byName := map[string]*apiv3.AuthConfig{}
	items := make([]*apiv3.AuthConfig, 0, len(configs))
	for _, cfg := range configs {
		b, err := json.Marshal(withTypeMeta(cfg))
		require.NoError(t, err)
		authConfig := &apiv3.AuthConfig{}
		require.NoError(t, json.Unmarshal(b, authConfig))
		require.NotEmpty(t, authConfig.Name, "AuthConfig fixture must have metadata.name")
		byName[authConfig.Name] = authConfig
		items = append(items, authConfig)
	}

	return &fakes.AuthConfigListerMock{
		GetFunc: func(_, name string) (*apiv3.AuthConfig, error) {
			if authConfig, ok := byName[name]; ok {
				return authConfig.DeepCopy(), nil
			}
			return nil, apierrors.NewNotFound(apiv3.Resource("authconfigs"), name)
		},
		ListFunc: func(_ string, selector labels.Selector) ([]*apiv3.AuthConfig, error) {
			var out []*apiv3.AuthConfig
			for _, authConfig := range items {
				if selector.Matches(labels.Set(authConfig.Labels)) {
					out = append(out, authConfig.DeepCopy())
				}
			}
			return out, nil
		},
	}
}

// New returns an AuthConfig client and lister that both serve the given
// AuthConfigs. See NewClient and NewLister.
func New(t *testing.T, configs ...map[string]any) (normanv3.AuthConfigInterface, normanv3.AuthConfigLister) {
	t.Helper()
	return NewClient(t, configs...), NewLister(t, configs...)
}

// ConcurrentWriteAnnotation is set on the AuthConfig by UpdateServer when it
// simulates a write by something else.
const ConcurrentWriteAnnotation = "authconfigtest.cattle.io/concurrent-write"

// UpdateServer is a fake API server for a single AuthConfig that serves Get
// and Update. Like a real API server, it rejects an Update with a Conflict
// unless its resourceVersion is the current one, and returns NotFound for any
// other AuthConfig.
type UpdateServer struct {
	client normanv3.AuthConfigInterface

	mu              sync.Mutex
	stored          map[string]any
	resourceVersion int
	writesAfterGets int
	updates         int
}

// NewUpdateServer starts an UpdateServer that serves cfg, which must have
// metadata.name set.
func NewUpdateServer(t *testing.T, cfg map[string]any) *UpdateServer {
	t.Helper()

	// Copy cfg, as the server modifies the stored AuthConfig.
	b, err := json.Marshal(withTypeMeta(cfg))
	require.NoError(t, err)
	s := &UpdateServer{resourceVersion: 1}
	require.NoError(t, json.Unmarshal(b, &s.stored))
	name, _ := s.stored["metadata"].(map[string]any)["name"].(string)
	require.NotEmpty(t, name, "AuthConfig fixture must have metadata.name")
	s.setResourceVersion()
	path := authConfigsPath + "/" + name

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if r.URL.Path != path {
			if !strings.HasPrefix(r.URL.Path, authConfigsPath+"/") {
				t.Errorf("unexpected AuthConfig API request: %s %s", r.Method, r.URL.Path)
			}
			writeStatus(w, http.StatusNotFound, "NotFound", strings.TrimPrefix(r.URL.Path, authConfigsPath+"/"))
			return
		}

		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, s.stored)
			if s.writesAfterGets > 0 {
				s.writesAfterGets--
				s.concurrentWrite()
			}
		case http.MethodPut:
			s.updates++
			var obj map[string]any
			if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
				t.Errorf("decoding AuthConfig update: %v", err)
				writeStatus(w, http.StatusBadRequest, "BadRequest", name)
				return
			}
			metadata, _ := obj["metadata"].(map[string]any)
			if metadata["resourceVersion"] != strconv.Itoa(s.resourceVersion) {
				writeStatus(w, http.StatusConflict, string(metav1.StatusReasonConflict), name)
				return
			}
			s.stored = obj
			s.resourceVersion++
			s.setResourceVersion()
			writeJSON(w, http.StatusOK, s.stored)
		default:
			t.Errorf("unexpected AuthConfig API request: %s %s", r.Method, r.URL.Path)
			writeStatus(w, http.StatusMethodNotAllowed, "MethodNotAllowed", name)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := normanv3.NewForConfig(rest.Config{Host: srv.URL})
	require.NoError(t, err)
	s.client = client.AuthConfigs("")

	return s
}

// Client returns an AuthConfig client for the server.
func (s *UpdateServer) Client() normanv3.AuthConfigInterface {
	return s.client
}

// WriteAfterGets makes the server act as if something else writes the
// AuthConfig after each of the next n Gets, so that an Update with the
// resourceVersion from one of those Gets conflicts. The write sets the
// ConcurrentWriteAnnotation.
func (s *UpdateServer) WriteAfterGets(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writesAfterGets = n
}

// Updates returns how many Updates the server has received.
func (s *UpdateServer) Updates() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updates
}

// Stored returns the AuthConfig as currently stored.
func (s *UpdateServer) Stored() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stored
}

func (s *UpdateServer) concurrentWrite() {
	metadata := s.stored["metadata"].(map[string]any)
	annotations, _ := metadata["annotations"].(map[string]any)
	if annotations == nil {
		annotations = map[string]any{}
	}
	annotations[ConcurrentWriteAnnotation] = "true"
	metadata["annotations"] = annotations
	s.resourceVersion++
	s.setResourceVersion()
}

func (s *UpdateServer) setResourceVersion() {
	metadata := s.stored["metadata"].(map[string]any)
	metadata["resourceVersion"] = strconv.Itoa(s.resourceVersion)
}

// NewSecretController returns a SecretController for which no Secrets exist
// to begin with. The data of each Secret created with it is recorded by name in
// the returned map. A Secret can only be created once.
func NewSecretController(t *testing.T) (wcorev1.SecretController, map[string]map[string]string) {
	t.Helper()

	created := map[string]map[string]string{}
	ctrl := gomock.NewController(t)
	cache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	cache.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(_, name string) (*corev1.Secret, error) {
		return nil, apierrors.NewNotFound(corev1.Resource("secrets"), name)
	}).AnyTimes()
	secrets := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().Cache().Return(cache).AnyTimes()
	secrets.EXPECT().Create(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		if _, ok := created[secret.Name]; ok {
			return nil, apierrors.NewAlreadyExists(corev1.Resource("secrets"), secret.Name)
		}
		created[secret.Name] = secret.StringData
		return secret, nil
	}).AnyTimes()

	return secrets, created
}

// RequireUpdatedAfterConflict fails the test unless the AuthConfig was updated
// after an Update that conflicted with a write set up by WriteAfterGets(1), and
// the write's annotation was kept.
func RequireUpdatedAfterConflict(t *testing.T, s *UpdateServer) {
	t.Helper()

	require.Equal(t, 2, s.Updates(), "the conflicting update should be retried once")
	metadata := s.Stored()["metadata"].(map[string]any)
	annotations, _ := metadata["annotations"].(map[string]any)
	require.Equal(t, "true", annotations[ConcurrentWriteAnnotation], "the update should keep the concurrent write's metadata")
}

// LDAPServer is a TCP listener that stands in for an LDAP or Active Directory
// server. It accepts and immediately closes connections, and counts them.
type LDAPServer struct {
	Host  string
	Port  int64
	dials atomic.Int32
}

// NewLDAPServer starts an LDAPServer that is stopped when the test ends.
func NewLDAPServer(t *testing.T) *LDAPServer {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	addr := l.Addr().(*net.TCPAddr)
	s := &LDAPServer{Host: addr.IP.String(), Port: int64(addr.Port)}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			s.dials.Add(1)
			_ = conn.Close()
		}
	}()

	return s
}

// Dials returns how many connections the server has accepted.
func (s *LDAPServer) Dials() int {
	return int(s.dials.Load())
}

func withTypeMeta(cfg map[string]any) map[string]any {
	out := make(map[string]any, len(cfg)+2)
	for k, v := range cfg {
		out[k] = v
	}
	out["apiVersion"] = "management.cattle.io/v3"
	out["kind"] = "AuthConfig"
	return out
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeStatus(w http.ResponseWriter, code int, reason, name string) {
	writeJSON(w, code, map[string]any{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Failure",
		"reason":     reason,
		"code":       code,
		"details":    map[string]any{"name": name, "group": "management.cattle.io", "kind": "authconfigs"},
	})
}

// RequireDialed fails the test unless s accepts exactly one connection.
func RequireDialed(t *testing.T, s *LDAPServer) {
	t.Helper()
	require.Eventually(t, func() bool { return s.Dials() > 0 }, 2*time.Second, 10*time.Millisecond, "expected a connection to %s:%d", s.Host, s.Port)
	require.Equal(t, 1, s.Dials())
}

// RequireNotDialed fails the test if any of servers accepts a connection.
func RequireNotDialed(t *testing.T, servers ...*LDAPServer) {
	t.Helper()
	require.Never(t, func() bool {
		for _, s := range servers {
			if s.Dials() > 0 {
				return true
			}
		}
		return false
	}, 100*time.Millisecond, 10*time.Millisecond, "unexpected connection to a directory server")
}

// RequireLoginRejected fails the test unless err is written to the client with
// the expected status code and its body contains msg.
func RequireLoginRejected(t *testing.T, err error, statusCode int, msg string) {
	t.Helper()
	require.Error(t, err)

	rec := httptest.NewRecorder()
	util.ReturnAPIError(rec, err)

	assert.Equal(t, statusCode, rec.Code, "unexpected response: %s", rec.Body.String())

	body := rec.Body.String()
	require.Contains(t, body, msg)
}
