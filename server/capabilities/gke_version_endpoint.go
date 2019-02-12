package capabilities

import (
	"fmt"
	"net/http"
)

const (
	defaultCredentialEnv = "GOOGLE_APPLICATION_CREDENTIALS"
)

// NewGKEVersionsHandler creates a new GKEVersionsHandler
func NewGKEVersionsHandler() *GKEVersionHandler {
	return &GKEVersionHandler{}
}

// GKEVersionHandler for listing available Kubernetes versions in GKE
type GKEVersionHandler struct {
}

type errorResponse struct {
	Error string `json:"error"`
}

func (g *GKEVersionHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	body := preCheck(writer, req)
	if body == nil {
		return
	}

	zone := body.Zone
	if zone == "" {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("invalid zone"))
		return
	}

	client, err := getContainerServiceClient(req.Context(), body.Credentials)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	result, err := client.Projects.Zones.GetServerconfig(body.ProjectID, zone).Do()

	postCheck(writer, result, err)
}
