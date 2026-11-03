package scim_test

import (
	"context"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/scim"
	dashboardapisettings "github.com/rancher/rancher/pkg/controllers/dashboardapi/settings"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	tick = 200 * time.Millisecond
	// expiredBeforeStart is created and left to expire before the controller starts.
	expiredBeforeStart = "scim-token-expired-before-start"
)

type TokenExpirySuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	testEnv         *envtest.Environment
	wranglerContext *wrangler.Context
}

func TestTokenExpirySuite(t *testing.T) {
	suite.Run(t, new(TokenExpirySuite))
}

func (s *TokenExpirySuite) SetupSuite() {
	t := s.T()
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	require.NoError(t, err)

	common.RegisterCRDs(s.ctx, t, restCfg, crd.CRD{SchemaObject: v3.Setting{}, NonNamespace: true})

	s.wranglerContext, err = wrangler.NewContext(s.ctx, nil, restCfg)
	require.NoError(t, err)

	_, err = s.wranglerContext.Core.Namespace().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace.System}})
	require.NoError(t, err)

	// Use the settings provider Rancher uses, so the controller reads the setting from the cache.
	require.NoError(t, dashboardapisettings.Register(s.wranglerContext.Mgmt.Setting()))

	scim.RegisterTokenExpiry(s.ctx, s.wranglerContext.Core.Secret(), s.wranglerContext.Mgmt.Setting())

	// The Setting controller and cache must run before setTTL can see its own change.
	common.StartWranglerControllers(s.ctx, t, s.wranglerContext, schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "Setting",
	})

	// The API server sets the creation time, so a Secret that expired before the controller started
	// has to be created and left to expire first.
	s.setTTL("1s")
	s.createSecret(namespace.System, expiredBeforeStart, true)
	time.Sleep(2 * time.Second)

	common.StartWranglerControllers(s.ctx, t, s.wranglerContext, schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	})
}

func (s *TokenExpirySuite) TearDownSuite() {
	s.cancel()
	require.NoError(s.T(), s.testEnv.Stop())
}

// setTTL sets the expiry setting and waits until the settings provider returns the new value.
func (s *TokenExpirySuite) setTTL(value string) {
	t := s.T()

	setting, err := s.wranglerContext.Mgmt.Setting().Get(settings.ExpireSCIMTokensAfter.Name, metav1.GetOptions{})
	require.NoError(t, err)
	setting = setting.DeepCopy()
	setting.Value = value
	_, err = s.wranglerContext.Mgmt.Setting().Update(setting)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return settings.ExpireSCIMTokensAfter.Get() == value
	}, 5*time.Second, tick)
}

func (s *TokenExpirySuite) createSecret(ns, name string, token bool) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"authn.management.cattle.io/provider": "okta"},
		},
		Data: map[string][]byte{"token": []byte("token")},
	}
	if token {
		secret.Labels["cattle.io/kind"] = "scim-auth-token"
	}

	_, err := s.wranglerContext.Core.Secret().Create(secret)
	require.NoError(s.T(), err)
}

func (s *TokenExpirySuite) exists(ns, name string) bool {
	_, err := s.wranglerContext.Core.Secret().Get(ns, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(s.T(), err)

	return true
}

func (s *TokenExpirySuite) TestDeletesSecretExpiredBeforeStart() {
	assert.Eventually(s.T(), func() bool {
		return !s.exists(namespace.System, expiredBeforeStart)
	}, 10*time.Second, tick)
}

func (s *TokenExpirySuite) TestDeletesSecretWhenItExpires() {
	t := s.T()
	const name = "scim-token-expires"

	// The API server truncates the creation time to the second, so the Secret can expire up to a
	// second early. The 2s window leaves a margin of at least a second.
	s.setTTL("4s")
	s.createSecret(namespace.System, name, true)

	assert.Never(t, func() bool {
		return !s.exists(namespace.System, name)
	}, 2*time.Second, tick, "deleted before it expired")
	assert.Eventually(t, func() bool {
		return !s.exists(namespace.System, name)
	}, 10*time.Second, tick, "not deleted after it expired")
}

func (s *TokenExpirySuite) TestShorterSettingDeletesSecret() {
	t := s.T()
	const name = "scim-token-shorter-setting"

	s.setTTL("1h")
	s.createSecret(namespace.System, name, true)

	assert.Never(t, func() bool {
		return !s.exists(namespace.System, name)
	}, 2*time.Second, tick, "deleted before the setting changed")

	s.setTTL("1s")

	assert.Eventually(t, func() bool {
		return !s.exists(namespace.System, name)
	}, 10*time.Second, tick, "not deleted after the setting changed")
}

func (s *TokenExpirySuite) TestKeepsOtherSecrets() {
	t := s.T()
	const (
		token          = "scim-token-control"
		otherNamespace = "scim-token-other-namespace"
		noKindLabel    = "scim-token-no-kind-label"
	)

	s.setTTL("1s")
	s.createSecret(metav1.NamespaceDefault, otherNamespace, true)
	s.createSecret(namespace.System, noKindLabel, false)
	s.createSecret(namespace.System, token, true)

	// Once the token Secret created last is deleted, the others are past their expiry too.
	require.Eventually(t, func() bool {
		return !s.exists(namespace.System, token)
	}, 10*time.Second, tick)

	assert.True(t, s.exists(metav1.NamespaceDefault, otherNamespace), "token Secret in another namespace was deleted")
	assert.True(t, s.exists(namespace.System, noKindLabel), "Secret without the kind label was deleted")
}

func (s *TokenExpirySuite) TestZeroSettingKeepsSecrets() {
	t := s.T()
	const name = "scim-token-zero-setting"

	s.setTTL("0")
	s.createSecret(namespace.System, name, true)

	assert.Never(t, func() bool {
		return !s.exists(namespace.System, name)
	}, 3*time.Second, tick, "deleted although tokens don't expire")
}
