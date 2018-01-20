package capabilities

import (
	"net/http"
	"context"
	"google.golang.org/api/container/v1"
	"encoding/json"
	"fmt"
)

const (
	defaultCredentialEnv = "GOOGLE_APPLICATION_CREDENTIALS"
)

func NewGKEVersionsHandler() *gkeVersionHandler {
	return &gkeVersionHandler{}
}

type gkeVersionHandler struct {
}

type errorResponse struct {
	Error string `json:"error"`
}

func (g *gkeVersionHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
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

	zone := body.Zone
	if zone == "" {
		writer.WriteHeader(http.StatusBadRequest)
		handleErr(writer, fmt.Errorf("invalid zone"))
		return
	}

	client, err := g.getServiceClient(context.Background(), body.Credentials)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	result, err := client.Projects.Zones.GetServerconfig(body.ProjectId, zone).Do()

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

func (g *gkeVersionHandler) getServiceClient(ctx context.Context, credentialContent string) (*container.Service, error) {
	client, err := getOAuthClient(ctx, credentialContent)

	if err != nil {
		return nil, err
	}

	service, err := container.New(client)

	if err != nil {
		return nil, err
	}
	return service, nil
}
