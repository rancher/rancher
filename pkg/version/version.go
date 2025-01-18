// Package version gathers version data from variables set at build time and runtime
// and provides access points to retrieve them.
package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/rancher/rancher/pkg/buildconfig"
)

var (
	Version   = "dev"
	GitCommit = "HEAD"
)

const primeEnv = "RANCHER_PRIME"

// Info encapsulates version metadata.
type Info struct {
	Version              string
	GitCommit            string
	RancherPrime         string
	MinKubernetesVersion string
	MaxKubernetesVersion string
}

// FriendlyVersion returns a human-readable string that can be included in log output.
func FriendlyVersion() string {
	return fmt.Sprintf("%s (%s)", Version, GitCommit)
}

type versionHandler struct {
	info Info
}

// NewVersionHandler checks the runtime environment for the RANCHER_PRIME environment variable
// and uses that along with build-time version information to create an HTTP handler.
func NewVersionHandler() http.Handler {
	var rancherPrime = "false"
	if isPrime, ok := os.LookupEnv(primeEnv); ok && isPrime == "true" {
		rancherPrime = "true"
	}
	return &versionHandler{info: Info{
		Version,
		GitCommit,
		rancherPrime,
		buildconfig.MinKubernetesVersion,
		buildconfig.MaxKubernetesVersion,
	}}
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
