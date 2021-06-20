package server

import (
	"net/http"

	"github.com/rancher/rancher/pkg/provisioningv2/rke2/installer"
)

func InstallHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		content, err := installer.InstallScript("", nil)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "text/plain")
		rw.Write(content)
	})
}
