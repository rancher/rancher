package capabilities

import (
	"fmt"
	"net/http"
)

// NewGKESubnetworksHandler creates a new GKESubnetworksHandler
func NewGKESubnetworksHandler() *GKESubnetworksHandler {
	return &GKESubnetworksHandler{}
}

// GKESubnetworksHandler lists available networks in GKE
type GKESubnetworksHandler struct {
}

type subnetworkCapabilitiesRequestBody struct {
	capabilitiesRequestBody

	Region string `json:"region"`
}

func (g *GKESubnetworksHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	body := preCheck(writer, req)
	if body == nil {
		return
	}

	if body.Region == "" {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("invalid region"))
		return
	}

	client, err := getComputeServiceClient(req.Context(), body.Credentials)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}
	result, err := client.Subnetworks.List(body.ProjectID, body.Region).Do()

	postCheck(writer, result, err)
}
