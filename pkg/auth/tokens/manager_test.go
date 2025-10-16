package tokens

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type DummyIndexer struct {
	cache.Store

	hashedEnabled bool
}

type TestCase struct {
	token   string
	userID  string
	receive bool
	err     string
}

var (
	token       string
	tokenHashed string
)

type TestManager struct {
	assert       *assert.Assertions
	tokenManager Manager
	apiCtx       *types.APIContext
	testCases    []TestCase
}

// TestTokenStreamTransformer validates that the function properly filters data in websocket
func TestTokenStreamTransformer(t *testing.T) {
	features.TokenHashing.Set(false)

	testManager := TestManager{
		assert: assert.New(t),
		tokenManager: Manager{
			tokenIndexer: &DummyIndexer{
				Store: &cache.FakeCustomStore{},
			},
		},
		apiCtx: &types.APIContext{
			Request: &http.Request{},
		},
	}

	var err error
	token, err = randomtoken.Generate()
	if err != nil {
		testManager.assert.FailNow(fmt.Sprintf("unable to generate token for token stream transformer test: %v", err))
	}
	sha256Hasher := hashers.Sha256Hasher{}
	tokenHashed, err = sha256Hasher.CreateHash(token)
	if err != nil {
		testManager.assert.FailNow(fmt.Sprintf("unable to hash token for token stream transformer test: %v", err))
	}

	testManager.testCases = []TestCase{
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
			err:     "Invalid auth token value",
		},
		{
			token:   "wrongname:testkey",
			userID:  "testuser",
			receive: false,
			err:     "422: [TokenStreamTransformer] failed: Invalid auth token value",
		},
		{
			token:   "testname:wrongkey",
			userID:  "testname",
			receive: false,
			err:     "422: [TokenStreamTransformer] failed: Invalid auth token value",
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

	testManager.runTestCases(false)
	testManager.runTestCases(true)
}

func (t *TestManager) runTestCases(hashingEnabled bool) {
	features.TokenHashing.Set(hashingEnabled)
	t.tokenManager = Manager{
		tokenIndexer: &DummyIndexer{
			Store:         &cache.FakeCustomStore{},
			hashedEnabled: hashingEnabled,
		},
	}
	for index, testCase := range t.testCases {
		failureMessage := fmt.Sprintf("test case #%d failed", index)

		dataStream := make(chan map[string]interface{}, 1)
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
		dataStream <- map[string]interface{}{"labels": map[string]interface{}{UserIDLabel: testCase.userID}}
		t.assert.Equal(<-dataReceived, testCase.receive)
		close(dataStream)
		ticker.Stop()
	}
}

func receivedData(c <-chan map[string]interface{}, t <-chan time.Time, result chan<- bool) {
	select {
	case <-c:
		result <- true
	case <-t:
		// assume data will not be received after 1 second timeout
		result <- false
	}
}

func (d *DummyIndexer) Index(indexName string, obj interface{}) ([]interface{}, error) {
	return nil, nil
}

func (d *DummyIndexer) IndexKeys(indexName, indexKey string) ([]string, error) {
	return []string{}, nil
}

func (d *DummyIndexer) ListIndexFuncValues(indexName string) []string {
	return []string{}
}

func (d *DummyIndexer) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	token := &apiv3.Token{
		Token: token,
		ObjectMeta: v1.ObjectMeta{
			Name: "testname",
		},
		UserID: "testuser",
	}
	if d.hashedEnabled {
		token.Annotations = map[string]string{TokenHashed: strconv.FormatBool(d.hashedEnabled)}
		token.Token = tokenHashed
	}
	return []interface{}{
		token,
	}, nil
}

func (d *DummyIndexer) GetIndexers() cache.Indexers {
	return nil
}

func (d *DummyIndexer) AddIndexers(newIndexers cache.Indexers) error {
	return nil
}

func (d *DummyIndexer) SetTokenHashed(enabled bool) {
	d.hashedEnabled = enabled
}
