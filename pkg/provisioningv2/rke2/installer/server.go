package installer

import (
	"net/http"
)

type handler struct{}

var Handler *handler

func (s *handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error
	var content []byte

	switch req.URL.Path {
	case SystemAgentInstallPath:
		content, err = LinuxInstallScript(req.Context(), "", nil, req.Host, nil)
	case WindowsRke2InstallPath:
		content, err = WindowsInstallScript(req.Context(), "", nil, req.Host, nil)
	}

	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "text/plain")
	rw.Write(content)
}
