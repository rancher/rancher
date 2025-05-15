// Package version gathers version data from variables set at build time and runtime
// and provides access points to retrieve them.
package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

var (
	Version   = "dev"
	GitCommit = "HEAD"
)

const primeEnv = "RANCHER_VERSION_TYPE"

// Info encapsulates version metadata.
type Info struct {
	Version      string
	GitCommit    string
	RancherPrime string
}

// FriendlyVersion returns a human-readable string that can be included in log output.
func FriendlyVersion() string {
	return fmt.Sprintf("%s (%s)", Version, GitCommit)
}

type versionHandler struct {
	info Info
}

// NewVersionHandler checks the runtime environment for the RANCHER_VERSION_TYPE environment variable
// and uses that along with build-time version information to create an HTTP handler.
func NewVersionHandler() http.Handler {
	rancherPrime := "false"
	if versionType, ok := os.LookupEnv(primeEnv); ok && versionType == "prime" {
		rancherPrime = "true"
	}
	return &versionHandler{info: Info{Version, GitCommit, rancherPrime}}
}

// ServeHTTP handles GET requests for version information.
func (h *versionHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	body, err := json.Marshal(h.info)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	rw.Write(body)
	rw.WriteHeader(http.StatusOK)
}
