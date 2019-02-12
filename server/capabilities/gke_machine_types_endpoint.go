package capabilities

import (
	"net/http"
)

// NewGKEMachineTypesHandler creates a new GKEMachineTypesHandler
func NewGKEMachineTypesHandler() *GKEMachineTypesHandler {
	return &GKEMachineTypesHandler{}
}

// GKEMachineTypesHandler lists available machine types in GKE
type GKEMachineTypesHandler struct {
	Field string
}

func (g *GKEMachineTypesHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
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

	result, err := client.MachineTypes.List(body.ProjectID, body.Zone).Do()

	postCheck(writer, result, err)
}
