package capabilities

import (
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
	body := preCheck(writer, req)
	if body == nil {
		return
	}

	client, err := getIamServiceClient(req.Context(), body.Credentials)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	name := "projects/" + body.ProjectID
	result, err := client.Projects.ServiceAccounts.List(name).Do()

	postCheck(writer, result, err)
}
