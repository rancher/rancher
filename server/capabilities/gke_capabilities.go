package capabilities

import (
	"github.com/rancher/kontainer-engine/drivers/gke"
	"os"
	"strings"
	"golang.org/x/oauth2/google"
	"context"
	"net/http"
	"io/ioutil"
	"io"
	"google.golang.org/api/container/v1"
	"golang.org/x/oauth2"
	"github.com/sirupsen/logrus"
	"encoding/json"
	"fmt"
)

type capabilitiesRequestBody struct {
	Credentials string `json:"credentials"`
	ProjectId   string `json:"projectId"`
}

func validateRequestBody(writer http.ResponseWriter, body *capabilitiesRequestBody) error {
	credentials := body.Credentials
	projectID := body.ProjectId

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

func extractRequestBody(writer http.ResponseWriter, req *http.Request, body interface{}) error {
	raw, err := ioutil.ReadAll(req.Body)

	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("cannot read request body: " + err.Error())
	}

	err = json.Unmarshal(raw, &body)

	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("cannot parse request body: " + err.Error())
	}

	return nil
}

func getOAuthClient(ctx context.Context, credentialContent string) (*http.Client, error) {
	// The google SDK has no sane way to pass in a TokenSource give all the different types (user, service account, etc)
	// So we actually set an environment variable and then unset it
	gke.EnvMutex.Lock()
	locked := true
	setEnv := false
	cleanup := func() {
		if setEnv {
			os.Unsetenv(defaultCredentialEnv)
		}

		if locked {
			gke.EnvMutex.Unlock()
			locked = false
		}
	}
	defer cleanup()

	file, err := ioutil.TempFile("", "credential-file")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()

	if _, err := io.Copy(file, strings.NewReader(credentialContent)); err != nil {
		return nil, err
	}

	setEnv = true
	os.Setenv(defaultCredentialEnv, file.Name())

	ts, err := google.DefaultTokenSource(ctx, container.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	// Unlocks
	cleanup()

	return oauth2.NewClient(ctx, ts), nil
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
