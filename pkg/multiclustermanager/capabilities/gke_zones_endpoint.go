package capabilities

import (
	"net/http"
)

// NewGKEZonesHandler creates a new GKEZonesHandler
func NewGKEZonesHandler() *GKEZonesHandler {
	return &GKEZonesHandler{}
}

// GKEZonesHandler for listing available GKE zones
type GKEZonesHandler struct {
	Field string
}

func (g *GKEZonesHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	body := preCheck(writer, req)
	if body == nil {
		return
	}
	client, err := getComputeServiceClient(req.Context(), body.Credentials)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	result, err := client.Zones.List(body.ProjectID).Do()

	postCheck(writer, result, err)
}
