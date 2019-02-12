package capabilities

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/api/iam/v1"

	"net/http"
)

// NewGKEServiceAccountsHandler creates a new GKEServiceAccountsHandler
func NewGKEServiceAccountsHandler() *GKEServiceAccountsHandler {
	return &GKEServiceAccountsHandler{}
}

// GKEServiceAccountsHandler lists available service accounts in GKE
type GKEServiceAccountsHandler struct {
}

func (g *GKEServiceAccountsHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
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

	name := "projects/" + body.ProjectID
	result, err := client.Projects.ServiceAccounts.List(name).Do()

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

func (g *GKEServiceAccountsHandler) getServiceClient(ctx context.Context, credentialContent string) (*iam.Service, error) {
	client, err := getOAuthClient(ctx, credentialContent)

	if err != nil {
		return nil, err
	}

	service, err := iam.New(client)

	if err != nil {
		return nil, err
	}
	return service, nil
}
