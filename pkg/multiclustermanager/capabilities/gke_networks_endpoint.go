package capabilities

import (
	"net/http"
)

// NewGKENetworksHandler creates a new GKENetworksHandler
func NewGKENetworksHandler() *GKENetworksHandler {
	return &GKENetworksHandler{}
}

// GKENetworksHandler lists available networks in GKE
type GKENetworksHandler struct {
}

func (g *GKENetworksHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
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

	result, err := client.Networks.List(body.ProjectID).Do()

	postCheck(writer, result, err)
}
