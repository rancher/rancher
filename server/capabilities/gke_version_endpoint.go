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
	region := body.Region
	if zone == "" && region == "" {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("invalid zone or region"))
		return
	}
	if zone != "" && region != "" {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("only one of region or zone can be provided"))
		return
	}

	client, err := getContainerServiceClient(req.Context(), body.Credentials)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	var location string
	if region != "" {
		location = region
	} else {
		location = zone
	}
	parent := "projects/" + body.ProjectID + "/locations/" + location

	result, err := client.Projects.Locations.GetServerConfig(parent).Do()

	postCheck(writer, result, err)
}
