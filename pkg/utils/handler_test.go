package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIBodyLimitingHandler(t *testing.T) {
	b, err := json.Marshal(map[string]any{
		"testing": "value",
		"multiple": []string{
			"test1",
			"test2",
			"test3",
		},
	})
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("reading body: %s", err), http.StatusBadRequest)
		}
	})

	t.Run("when the limit is smaller than the body", func(t *testing.T) {
		limitingHandler := APIBodyLimitingHandler(int64(len(b) - 2))
		require.NoError(t, err)

		srv := httptest.NewServer(limitingHandler(handler))
		defer srv.Close()

		resp, err := srv.Client().Post(srv.URL, "application/json", bytes.NewReader(b))
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("when the limit is larger than the body", func(t *testing.T) {
		limitingHandler := APIBodyLimitingHandler(int64(len(b)))
		require.NoError(t, err)

		srv := httptest.NewServer(limitingHandler(handler))
		defer srv.Close()

		resp, err := srv.Client().Post(srv.URL, "application/json", bytes.NewReader(b))
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
