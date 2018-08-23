package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/sirupsen/logrus"
)

var (
	clients = map[string]*http.Client{}
	l       sync.Mutex
	counter int64
)

func authorizer(req *http.Request) (string, bool, error) {
	id := req.Header.Get("x-tunnel-id")
	return id, id != "", nil
}

func Client(server *remotedialer.Server, rw http.ResponseWriter, req *http.Request) {
	timeout := req.URL.Query().Get("timeout")
	if timeout == "" {
		timeout = "15"
	}

	vars := mux.Vars(req)
	clientKey := vars["id"]
	url := fmt.Sprintf("%s://%s%s", vars["scheme"], vars["host"], vars["path"])
	client := getClient(server, clientKey, timeout)

	id := atomic.AddInt64(&counter, 1)
	logrus.Infof("[%03d] REQ t=%s %s", id, timeout, url)

	resp, err := client.Get(url)
	if err != nil {
		logrus.Errorf("[%03d] REQ ERR t=%s %s: %v", id, timeout, url, err)
		remotedialer.DefaultErrorWriter(rw, req, 500, err)
		return
	}
	defer resp.Body.Close()

	logrus.Infof("[%03d] REQ OK t=%s %s", id, timeout, url)
	rw.WriteHeader(resp.StatusCode)
	io.Copy(rw, resp.Body)
	logrus.Infof("[%03d] REQ DONE t=%s %s", id, timeout, url)
}

func getClient(server *remotedialer.Server, clientKey, timeout string) *http.Client {
	l.Lock()
	defer l.Unlock()

	key := fmt.Sprintf("%s/%s", clientKey, timeout)
	client := clients[key]
	if client != nil {
		return client
	}

	dialer := server.Dialer(clientKey, 15*time.Second)
	client = &http.Client{
		Transport: &http.Transport{
			Dial: dialer,
		},
	}
	if timeout != "" {
		t, err := strconv.Atoi(timeout)
		if err == nil {
			client.Timeout = time.Duration(t) * time.Second
		}
	}

	clients[key] = client
	return client
}

func main() {
	var (
		addr      string
		peerID    string
		peerToken string
		peers     string
		debug     bool
	)
	flag.StringVar(&addr, "listen", ":8123", "Listen address")
	flag.StringVar(&peerID, "id", "", "Peer ID")
	flag.StringVar(&peerToken, "token", "", "Peer Token")
	flag.StringVar(&peers, "peers", "", "Peers format id:token:url,id:token:url")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.Parse()

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
		remotedialer.PrintTunnelData = true
	}

	handler := remotedialer.New(authorizer, remotedialer.DefaultErrorWriter)
	handler.PeerToken = peerToken
	handler.PeerID = peerID

	for _, peer := range strings.Split(peers, ",") {
		parts := strings.SplitN(strings.TrimSpace(peer), ":", 3)
		if len(parts) != 3 {
			continue
		}
		handler.AddPeer(parts[2], parts[0], parts[1])
	}

	router := mux.NewRouter()
	router.Handle("/connect", handler)
	router.HandleFunc("/client/{id}/{scheme}/{host}{path:.*}", func(rw http.ResponseWriter, req *http.Request) {
		Client(handler, rw, req)
	})

	fmt.Println("Listening on ", addr)
	http.ListenAndServe(addr, router)
}
