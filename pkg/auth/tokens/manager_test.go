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
			token:   "testname:testkey",
			userID:  "testuser",
			receive: true,
			err:     "",
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
			token:   "testname:testkey",
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

	for _, testCase := range testCases {
		dataStream := make(chan map[string]interface{}, 1)
		dataReceived := make(chan bool, 1)

		apiCtx.Request.Header = map[string][]string{"Authorization": {fmt.Sprintf("Bearer %s", testCase.token)}}

		df, err := tokenManager.TokenStreamTransformer(apiCtx, nil, dataStream, nil)
		if testCase.err == "" {
			assert.Nil(err)
		} else {
			assert.NotNil(err)
			assert.Contains(err.Error(), testCase.err)
		}

		ticker := time.NewTicker(1 * time.Second)
		go receivedData(df, ticker.C, dataReceived)

		// test data is received when data stream contains matching userID
		dataStream <- map[string]interface{}{"labels": map[string]interface{}{UserIDLabel: testCase.userID}}
		assert.Equal(<-dataReceived, testCase.receive)
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
			Token: "testkey",
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
