package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteResponse(t *testing.T) {
	t.Run("no payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeResponse(w, noPayload, http.StatusNoContent)

		assert.Equal(t, http.StatusNoContent, w.Result().StatusCode)
		assert.Equal(t, "", w.Result().Header.Get("Content-Type"))
		assert.Equal(t, 0, w.Body.Len())
	})
}
