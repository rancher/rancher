package capabilities

import (
	"net/http"
	"context"
	"google.golang.org/api/compute/v1"
	"encoding/json"
	"fmt"
)

func NewGKEMachineTypesHandler() *gkeMachineTypesHandler {
	return &gkeMachineTypesHandler{}
}

type gkeMachineTypesHandler struct {
	Field string
}

func (g *gkeMachineTypesHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writer.Header().Set("Content-Type", "application/json")

	var body zoneCapabilitiesRequestBody
	err := extractRequestBody(writer, req, &body)

	if err != nil {
		handleErr(writer, err)
		return
	}

	err = validateRequestBody(writer, &body.capabilitiesRequestBody)

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

	result, err := client.MachineTypes.List(body.ProjectId, body.Zone).Do()

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

func (g *gkeMachineTypesHandler) getServiceClient(ctx context.Context, credentialContent string) (*compute.Service, error) {
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
