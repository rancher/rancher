package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/iam/v1"
)

type capabilitiesRequestBody struct {
	Credentials string `json:"credentials,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Zone        string `json:"zone,omitempty"`
	Region      string `json:"region,omitempty"`
}

func validateCapabilitiesRequestBody(writer http.ResponseWriter, body *capabilitiesRequestBody) error {
	credentials := body.Credentials
	projectID := body.ProjectID

	if projectID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid projectId")
	}

	if credentials == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid credentials")
	}

	return nil
}

func getOAuthClient(ctx context.Context, credentialContent string) (*http.Client, error) {
	ts, err := google.CredentialsFromJSON(ctx, []byte(credentialContent), container.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	return oauth2.NewClient(ctx, ts.TokenSource), nil
}

func handleErr(writer http.ResponseWriter, originalErr error) {
	resp := errorResponse{originalErr.Error()}

	asJSON, err := json.Marshal(resp)

	if err != nil {
		logrus.Error("error while marshalling error message '" + originalErr.Error() + "' error was '" + err.Error() + "'")
		writer.Write([]byte(err.Error()))
		return
	}

	writer.Write([]byte(asJSON))
}

func preCheck(writer http.ResponseWriter, req *http.Request) *capabilitiesRequestBody {
	if req.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	writer.Header().Set("Content-Type", "application/json")

	var body capabilitiesRequestBody
	err := extractRequestBody(writer, req, &body)

	if err != nil {
		handleErr(writer, err)
		return nil
	}

	err = validateCapabilitiesRequestBody(writer, &body)

	if err != nil {
		handleErr(writer, err)
		return nil
	}
	return &body
}

func postCheck(writer http.ResponseWriter, result interface{}, err error) {
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	serialized, err := json.Marshal(&result)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		handleErr(writer, err)
		return
	}

	writer.Write(serialized)
}

func getComputeServiceClient(ctx context.Context, credentialContent string) (*compute.Service, error) {
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

func getIamServiceClient(ctx context.Context, credentialContent string) (*iam.Service, error) {
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

func getContainerServiceClient(ctx context.Context, credentialContent string) (*container.Service, error) {
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
