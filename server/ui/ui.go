package ui

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"crypto/tls"

	"github.com/rancher/norman/parse"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

var (
	insecureClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

func Content() http.Handler {
	return http.FileServer(http.Dir(settings.UIPath.Get()))
}

func UI(next http.Handler) http.Handler {
	local := false
	_, err := os.Stat(indexHTML())
	if err == nil {
		local = true
	}

	if local && !strings.HasPrefix(settings.ServerVersion.Get(), "v") {
		local = false
	}

	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			if local {
				http.ServeFile(resp, req, indexHTML())
			} else {
				ui(resp, req)
			}
		} else {
			next.ServeHTTP(resp, req)
		}
	})
}

func indexHTML() string {
	return filepath.Join(settings.UIPath.Get(), "index.html")
}

func ui(resp http.ResponseWriter, req *http.Request) {
	if err := serveIndex(resp, req); err != nil {
		logrus.Errorf("failed to serve UI: %v", err)
		resp.WriteHeader(500)
	}
}

func serveIndex(resp http.ResponseWriter, req *http.Request) error {
	r, err := insecureClient.Get(settings.UIIndex.Get())
	if err != nil {
		return err
	}
	defer r.Body.Close()

	_, err = io.Copy(resp, r.Body)
	return err
}
