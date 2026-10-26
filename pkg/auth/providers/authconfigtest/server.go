// Package authconfigtest provides test helpers for code that reads AuthConfigs.
package authconfigtest

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
