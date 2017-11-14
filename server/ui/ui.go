package ui

import (
	"io"
	"net/http"

	"github.com/rancher/norman/parse"
	"github.com/sirupsen/logrus"
)

var uiURL = "https://releases.rancher.com/ui/latest2/index.html"

func UI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			ui(resp, req)
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
