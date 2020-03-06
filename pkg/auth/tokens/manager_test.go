package tokens

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type DummyIndexer struct {
	cache.Store
}

type TestCase struct {
	name    string
	token   string
	userID  string
	receive bool
	err     string
}

// TestTokenStreamTransformer validates that the function properly filters data in websocket
func TestTokenStreamTransformer(t *testing.T) {
	assert := assert.New(t)

	tokenManager := Manager{
		tokenIndexer: &DummyIndexer{
			&cache.FakeCustomStore{},
		},
	}

	apiCtx := &types.APIContext{
		Request: &http.Request{},
	}

	testCases := []TestCase{
		{
			name:    "valid token",
			token:   "testname:testkey",
			userID:  "testuser",
			receive: true,
			err:     "",
		},
		{
			name:    "invalid token name",
			token:   "wrongname:testkey",
			userID:  "testuser",
			receive: false,
			err:     "422: [TokenStreamTransformer] failed: Invalid auth token value",
		},
		{
			name:    "invalid token key",
			token:   "testname:wrongkey",
			userID:  "testname",
			receive: false,
			err:     "422: [TokenStreamTransformer] failed: Invalid auth token value",
		},
		{
			name:    "invalid user for token",
			token:   "testname:testkey",
			userID:  "diffname",
			receive: false,
			err:     "",
		},
		{
			name:    "no token provided",
			token:   "",
			userID:  "testuser",
			receive: false,
			err:     "401: [TokenStreamTransformer] failed: No valid token cookie or auth header",
		},
	}

	for _, testCase := range testCases {
		dataStream := make(chan map[string]interface{}, 1)
		dataReceived := make(chan bool, 1)

		apiCtx.Request.Header = map[string][]string{"Authorization": {fmt.Sprintf("Bearer %s", testCase.token)}}

		df, err := tokenManager.TokenStreamTransformer(apiCtx, nil, dataStream, nil)
		if testCase.err == "" {
			assert.Nilf(err, "\"%s\" should not return an error but did", testCase.name)
		} else {
			assert.NotNilf(err, "\"%s\" should return an error but did not", testCase.name)
			assert.Containsf(err.Error(), testCase.err, "\"%s\" returned error that did not match", testCase.name)
		}

		ticker := time.NewTicker(1 * time.Second)
		go receivedData(df, ticker.C, dataReceived)

		// test data is received when data stream contains matching userID
		dataStream <- map[string]interface{}{"labels": map[string]interface{}{UserIDLabel: testCase.userID}}
		assert.Equalf(<-dataReceived, testCase.receive, "\"%s\" receive value \"%t\" was not correct", testCase.name, testCase.receive)
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
	return []interface{}{
		&v3.Token{
			Token: "$1:dGVzdHNhbHQ:THYNxKRchboM+OTTYgV2vjXO8T7GoIfkT+vkI7eWH60", // testsalt, testkey
			ObjectMeta: v1.ObjectMeta{
				Name: "testname",
			},
			UserID: "testuser",
		},
	}, nil
}

func (d *DummyIndexer) GetIndexers() cache.Indexers {
	return nil
}

func (d *DummyIndexer) AddIndexers(newIndexers cache.Indexers) error {
	return nil
}
