package ui

import (
	"io"
	"net/http"

	"os"

	"github.com/rancher/norman/parse"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

var uiURL = "https://releases.rancher.com/ui/latest2/index.html"

func Content() http.Handler {
	return http.FileServer(http.Dir("ui"))
}

func UI(next http.Handler) http.Handler {
	local := false
	_, err := os.Stat("ui/index.html")
	if err == nil {
		local = true
	}

	if local && settings.ServerVersion.Get() == "master" {
		local = false
	}

	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			if local {
				http.ServeFile(resp, req, "ui/index.html")
			} else {
				ui(resp, req)
			}
		} else {
			next.ServeHTTP(resp, req)
		}
	})
}

func ui(resp http.ResponseWriter, req *http.Request) {
	if err := serveIndex(resp, req); err != nil {
		logrus.Errorf("failed to serve UI: %v", err)
		resp.WriteHeader(500)
	}
}

func serveIndex(resp http.ResponseWriter, req *http.Request) error {
	r, err := http.Get(uiURL)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	_, err = io.Copy(resp, r.Body)
	return err
}
