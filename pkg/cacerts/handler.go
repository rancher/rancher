package cacerts

import (
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
)

func Handler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "text/plain")
	ca := settings.CACerts.Get()
	if strings.TrimSpace(ca) != "" {
		if !strings.HasSuffix(ca, "\n") {
			ca += "\n"
		}
		_, _ = rw.Write([]byte(ca))
	}
}
