package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/api/compute/v1"
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
	if req.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writer.Header().Set("Content-Type", "application/json")

	var body subnetworkCapabilitiesRequestBody
	err := extractRequestBody(writer, req, &body)

	if err != nil {
		handleErr(writer, err)
		return
	}

	err = validateCapabilitiesRequestBody(writer, &body.capabilitiesRequestBody)

	if err != nil {
		handleErr(writer, err)
		return
	}

	if body.Credentials == "" {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("invalid credentials"))
		return
	}

	client, err := g.getServiceClient(context.Background(), body.Credentials)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}
	result, err := client.Subnetworks.List(body.ProjectID, body.Region).Do()

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	serialized, err := json.Marshal(result)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	writer.Write(serialized)
}

func (g *GKESubnetworksHandler) getServiceClient(ctx context.Context, credentialContent string) (*compute.Service, error) {
	client, err := getOAuthClient(ctx, credentialContent)

	if err != nil {
		return nil, err
	}

	service, err := compute.New(client)

	if err != nil {
		return nil, err
	}
	return service, nil
}
