package remotedialer

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExceedBuffer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	producerAddress, err := newTestProducer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	serverAddress, server, err := newTestServer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := newTestClient(ctx, "ws://"+serverAddress); err != nil {
		t.Fatal(err)
	}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, proto, address string) (net.Conn, error) {
				return server.Dialer("client")(ctx, proto, address)
			},
		},
	}

	producerURL := "http://" + producerAddress

	resp, err := client.Get(producerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	resp2, err := client.Get(producerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	resp2Body, err := ioutil.ReadAll(resp2.Body)
	if err != nil {
		t.Fatal(err)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4096*4096, len(resp2Body))
	assert.Equal(t, 4096*4096, len(respBody))
}

func newTestServer(ctx context.Context) (string, *Server, error) {
	auth := func(req *http.Request) (clientKey string, authed bool, err error) {
		return "client", true, nil
	}

	server := New(auth, DefaultErrorWriter)
	address, err := newServer(ctx, server)
	return address, server, err
}

func newTestClient(ctx context.Context, url string) error {
	result := make(chan error, 2)
	go func() {
		err := ConnectToProxy(ctx, url, nil, func(proto, address string) bool {
			return true
		}, nil, func(ctx context.Context, session *Session) error {
			result <- nil
			return nil
		})
		result <- err
	}()
	return <-result
}

func newServer(ctx context.Context, handler http.Handler) (string, error) {
	server := http.Server{
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
		Handler: handler,
	}
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	go func() {
		<-ctx.Done()
		listener.Close()
		server.Shutdown(context.Background())
	}()
	go server.Serve(listener)
	return listener.Addr().String(), nil
}

func newTestProducer(ctx context.Context) (string, error) {
	buffer := make([]byte, 4096)
	return newServer(ctx, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for i := 0; i < 4096; i++ {
			if _, err := resp.Write(buffer); err != nil {
				panic(err)
			}
		}
	}))
}
