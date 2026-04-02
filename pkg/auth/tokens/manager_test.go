package tokens

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/mapper"
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

var (
	// This is shared state across the tests.
	token       string
	tokenHashed string
)

type TestManager struct {
	assert       *assert.Assertions
	tokenManager Manager
	apiCtx       *types.APIContext
	testCases    []testCase
}

func TestGetTokenLegacy(t *testing.T) {
	tokenName := "testname"
	tokenKey := mustGenerateRandomToken(t)
	legacyToken := &apiv3.Token{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Token",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: tokenName,
		},
		AuthProvider: "testing",
		Token:        tokenKey,
		TTLMillis:    0,
		UserID:       "u-mo12345",
	}
	di := &dummyIndexer{
		Store: &cache.FakeCustomStore{},
	}
	di.Empty()
	tokenManager := Manager{
		tokenIndexer: di,
		tokens: &fakeTokenClient{
			get: legacyToken,
		},
	}

	token, code, err := tokenManager.GetToken(tokenName + ":" + tokenKey)
	assert.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Equal(t, legacyToken, token.(*apiv3.Token))
}

func TestGetTokenExt(t *testing.T) {
	tokenName := "testname"
	tokenKey := "dddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	// SHA3 hash of tokenKey
	hashedTokenKey := "$3:1:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA"
	extUser := "u-mo12345"
	extToken := &ext.Token{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ext.cattle.io/v1",
			Kind:       "Token",
		},
		ObjectMeta: metav1.ObjectMeta{
			UID:  "kubid",
			Name: tokenName,
			Labels: map[string]string{
				"cattle.io/user-id": "u-mo12345",
			},
		},
		Spec: ext.TokenSpec{
			TTL:    -1,
			UserID: extUser,
			UserPrincipal: ext.TokenPrincipal{
				Name:     "world",
				Provider: "testing",
			},
			Kind:    "session",
			Enabled: ptr.To(false),
		},
		Status: ext.TokenStatus{
			Hash:           hashedTokenKey,
			LastUpdateTime: "13:00:05",
		},
	}
	extPrincipal := ext.TokenPrincipal{
		Name:     "world",
		Provider: "testing",
	}
	extPrincipalBytes, _ := json.Marshal(extPrincipal)
	extSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: tokenName,
			Labels: map[string]string{
				exttokenstore.UserIDLabel:     extUser,
				exttokenstore.SecretKindLabel: exttokenstore.SecretKindLabelValue,
			},
			UID: "",
		},
		Data: map[string][]byte{
			exttokenstore.FieldDescription:    []byte(""),
			exttokenstore.FieldEnabled:        []byte("false"),
			exttokenstore.FieldHash:           []byte(hashedTokenKey),
			exttokenstore.FieldKind:           []byte(exttokenstore.IsLogin),
			exttokenstore.FieldLastUpdateTime: []byte("13:00:05"),
			exttokenstore.FieldPrincipal:      extPrincipalBytes,
			exttokenstore.FieldTTL:            []byte("-1"),
			exttokenstore.FieldUID:            []byte("kubid"),
			exttokenstore.FieldUserID:         []byte(extUser),
		},
	}
	ctrl := gomock.NewController(t)
	secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
	users.EXPECT().Cache().Return(nil)
	secrets.EXPECT().Cache().Return(scache)
	scache.EXPECT().Get("cattle-tokens", tokenName).Return(&extSecret, nil)
	tokenManager := Manager{
		extTokenStore: exttokenstore.NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil),
	}

	token, code, err := tokenManager.GetToken("ext/" + tokenName + ":" + tokenKey)
	assert.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Equal(t, extToken, token.(*ext.Token))
}

func TestListTokens(t *testing.T) {
	tokenName := "testname"
	token = mustGenerateRandomToken(t)
	tokenManager := Manager{
		tokenIndexer: &dummyIndexer{
			Store: &cache.FakeCustomStore{},
		},
		tokens: &fakeTokenClient{
			// Two tokens one matches the token and is current
			// the other token does not match the token and is not current.
			list: []apiv3.Token{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "management.cattle.io/v3",
						Kind:       "Token",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: tokenName,
					},
					AuthProvider: "testing",
					Token:        token,
					TTLMillis:    0,
					UserID:       "u-mo12345",
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "management.cattle.io/v3",
						Kind:       "Token",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-" + tokenName,
					},
					AuthProvider: "testing",
					Token:        token,
					TTLMillis:    0,
					UserID:       "u-mo12345",
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v3/tokens", nil)
	w := &normanRecorder{}
	req.Header.Set("Authorization", "Bearer "+tokenName+":"+token)
	ac := &types.APIContext{ResponseWriter: w, Request: req, Schema: &types.Schema{Mapper: mapper.NewObject()}}

	if err := tokenManager.listTokens(ac); err != nil {
		t.Fatal(err)
	}

	want := []map[string]any{
		map[string]any{
			".selfLink":    "",
			"authProvider": "testing",
			"current":      true,
			"description":  "",
			"expired":      false,
			"expiresAt":    "",
			"isDerived":    false,
			"token":        token,
			"ttl":          json.Number("0"),
			"userId":       "u-mo12345",
			"userPrincipal": map[string]any{
				"metadata": map[string]any{},
			},
		},
		map[string]any{
			".selfLink":    "",
			"authProvider": "testing",
			"current":      false,
			"description":  "",
			"expired":      false,
			"expiresAt":    "",
			"isDerived":    false,
			"token":        token,
			"ttl":          json.Number("0"),
			"userId":       "u-mo12345",
			"userPrincipal": map[string]any{
				"metadata": map[string]any{},
			},
		},
	}
	assert.Len(t, w.Responses, 1)
	assert.Equal(t, want, w.Responses[0].Data)
}

// TestTokenStreamTransformer validates that the function properly filters data in websocket
func TestTokenStreamTransformer(t *testing.T) {
	features.TokenHashing.Set(false)

	testManager := TestManager{
		assert: assert.New(t),
		tokenManager: Manager{
			tokenIndexer: &dummyIndexer{
				Store: &cache.FakeCustomStore{},
			},
		},
		apiCtx: &types.APIContext{
			Request: &http.Request{},
		},
	}

	var err error
	token = mustGenerateRandomToken(t)
	sha256Hasher := hashers.Sha256Hasher{}
	tokenHashed, err = sha256Hasher.CreateHash(token)
	testManager.assert.NoError(err, "unable to hash token for token stream transformer test")

	testManager.testCases = []testCase{
		{
			token:   "testname:" + token,
			userID:  "testuser",
			receive: true,
			err:     "",
		},
		{
			token:   "testname:testtoken",
			userID:  "testuser",
			receive: false,
			err:     "invalid auth token value",
		},
		{
			token:   "wrongname:testkey",
			userID:  "testuser",
			receive: false,
			err:     "422: [TokenStreamTransformer] failed: invalid auth token value",
		},
		{
			token:   "testname:wrongkey",
			userID:  "testname",
			receive: false,
			err:     "422: [TokenStreamTransformer] failed: invalid auth token value",
		},
		{
			token:   "testname:" + token,
			userID:  "diffname",
			receive: false,
			err:     "",
		},
		{
			token:   "",
			userID:  "testuser",
			receive: false,
			err:     "401: [TokenStreamTransformer] failed: No valid token cookie or auth header",
		},
	}

	testManager.runtestCases(false)
	testManager.runtestCases(true)
}

func (t *TestManager) runtestCases(hashingEnabled bool) {
	features.TokenHashing.Set(hashingEnabled)
	t.tokenManager = Manager{
		tokenIndexer: &dummyIndexer{
			Store:         &cache.FakeCustomStore{},
			hashedEnabled: hashingEnabled,
		},
	}
	for index, testCase := range t.testCases {
		failureMessage := fmt.Sprintf("test case #%d failed", index)

		dataStream := make(chan map[string]any, 1)
		dataReceived := make(chan bool, 1)

		t.apiCtx.Request.Header = map[string][]string{"Authorization": {fmt.Sprintf("Bearer %s", testCase.token)}}

		df, err := t.tokenManager.TokenStreamTransformer(t.apiCtx, nil, dataStream, nil)
		if testCase.err == "" {
			t.assert.Nil(err, failureMessage)
		} else {
			t.assert.NotNil(err, failureMessage)
			t.assert.Contains(err.Error(), testCase.err, failureMessage)
		}

		ticker := time.NewTicker(1 * time.Second)
		go receivedData(df, ticker.C, dataReceived)

		// test data is received when data stream contains matching userID
		dataStream <- map[string]any{"labels": map[string]any{UserIDLabel: testCase.userID}}
		t.assert.Equal(<-dataReceived, testCase.receive)
		close(dataStream)
		ticker.Stop()
	}
}

func receivedData(c <-chan map[string]any, t <-chan time.Time, result chan<- bool) {
	select {
	case <-c:
		result <- true
	case <-t:
		// assume data will not be received after 1 second timeout
		result <- false
	}
}

type dummyIndexer struct {
	cache.Store

	hashedEnabled bool
	empty         bool
}

type testCase struct {
	token   string
	userID  string
	receive bool
	err     string
}

func (d *dummyIndexer) Index(indexName string, obj any) ([]any, error) {
	return nil, nil
}

func (d *dummyIndexer) IndexKeys(indexName, indexKey string) ([]string, error) {
	return []string{}, nil
}

func (d *dummyIndexer) ListIndexFuncValues(indexName string) []string {
	return []string{}
}

func (d *dummyIndexer) ByIndex(indexName, indexKey string) ([]any, error) {
	if d.empty {
		return nil, nil
	}

	token := &apiv3.Token{
		Token: token,
		ObjectMeta: metav1.ObjectMeta{
			Name: "testname",
		},
		UserID: "testuser",
	}
	if d.hashedEnabled {
		token.Annotations = map[string]string{TokenHashed: strconv.FormatBool(d.hashedEnabled)}
		token.Token = tokenHashed
	}
	return []any{
		token,
	}, nil
}

func (d *dummyIndexer) GetIndexers() cache.Indexers {
	return nil
}

func (d *dummyIndexer) AddIndexers(newIndexers cache.Indexers) error {
	return nil
}

func (d *dummyIndexer) SetTokenHashed(enabled bool) {
	d.hashedEnabled = enabled
}

func (d *dummyIndexer) Empty() {
	d.empty = true
}

func mustGenerateRandomToken(t *testing.T) string {
	t.Helper()
	tok, err := randomtoken.Generate()
	assert.NoError(t, err, "unable to generate token for token stream transformer test")

	return tok
}

type fakeTokenClient struct {
	list []apiv3.Token
	get  *apiv3.Token
}

func (f *fakeTokenClient) Create(o *apiv3.Token) (*apiv3.Token, error) {
	return nil, nil
}

func (f *fakeTokenClient) Get(name string, options metav1.GetOptions) (*apiv3.Token, error) {
	return f.get.DeepCopy(), nil
}

func (f *fakeTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return nil
}

func (f *fakeTokenClient) List(opts metav1.ListOptions) (*apiv3.TokenList, error) {
	return &apiv3.TokenList{Items: f.list}, nil
}

func (f *fakeTokenClient) Update(*apiv3.Token) (*apiv3.Token, error) {
	return nil, nil
}

// TODO: This should be moved to norman _or_ a test package in rancher.
// normanRecorder is like httptest.ResponseRecorder, but for norman's types.ResponseWriter interface
type normanRecorder struct {
	Responses []struct {
		Code int
		Data any
	}
}

func (n *normanRecorder) Write(apiContext *types.APIContext, code int, obj any) {
	if obj != nil {
		n.Responses = append(n.Responses, struct {
			Code int
			Data any
		}{
			Code: code,
			Data: obj,
		})
	}
}
