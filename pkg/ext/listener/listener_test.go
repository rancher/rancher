package listener

import (
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	addr = "localhost:8081"
)

func TestListener(t *testing.T) {
	ln, err := NewListener(addr)
	require.NoError(t, err)
	defer ln.Stop()

	err = ln.Start()
	require.NoError(t, err, assert.FailNow)

	srv := &http.Server{
		Addr: "",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello, World!"))
		}),
	}

	client := http.Client{
		Timeout: time.Second * 5,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("failed to serve: %s", err)
		}
	}()
	defer srv.Close()

	resp, err := client.Get("http://" + addr)
	assert.NoError(t, err)

	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(data), "Hello, World!")

	err = ln.Stop()
	require.NoError(t, err)

	err = ln.Start()
	require.NoError(t, err, assert.FailNow)

	assert.Eventually(t, func() bool {
		newResp, err := client.Get("http://" + addr)
		if !assert.NoError(t, err) {
			return false
		}

		ok := true

		newData, err := io.ReadAll(newResp.Body)
		ok = ok && assert.NoError(t, err)
		ok = ok && assert.Equal(t, string(newData), "Hello, World!")

		return ok
	}, time.Second*5, time.Millisecond*100)
}
