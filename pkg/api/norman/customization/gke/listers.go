package gke

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
	gkeapi "google.golang.org/api/container/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

func getTokenSource(ctx context.Context, credential string) (oauth2.TokenSource, error) {
	ts, err := google.CredentialsFromJSON(ctx, []byte(credential), gkeapi.CloudPlatformScope)
	if err != nil {
		return nil, err
	}
	return ts.TokenSource, nil
}

func getComputeServiceClient(ctx context.Context, credentialContent string) (*compute.Service, error) {
	ts, err := getTokenSource(ctx, credentialContent)
	if err != nil {
		return nil, err
	}

	service, err := compute.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, ts)))
	if err != nil {
		return nil, err
	}

	return service, nil
}

func getIamServiceClient(ctx context.Context, credentialContent string) (*iam.Service, error) {
	ts, err := getTokenSource(ctx, credentialContent)
	if err != nil {
		return nil, err
	}

	service, err := iam.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, ts)))
	if err != nil {
		return nil, err
	}

	return service, nil
}

func getContainerServiceClient(ctx context.Context, credentialContent string) (*container.Service, error) {
	ts, err := getTokenSource(ctx, credentialContent)
	if err != nil {
		return nil, err
	}

	service, err := container.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, ts)))
	if err != nil {
		return nil, err
	}

	return service, nil
}

func listMachineTypes(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ProjectID == "" || cap.Zone == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("projectId and zone are required")
	}

	client, err := getComputeServiceClient(ctx, cap.Credentials)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	result, err := client.MachineTypes.List(cap.ProjectID, cap.Zone).Do()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return encodeOutput(result)
}

func listNetworks(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ProjectID == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("projectId is required")
	}

	client, err := getComputeServiceClient(ctx, cap.Credentials)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	result, err := client.Networks.List(cap.ProjectID).Do()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return encodeOutput(result)
}

func listSubnetworks(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ProjectID == "" || cap.Region == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("projectId and region are required")
	}

	client, err := getComputeServiceClient(ctx, cap.Credentials)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	result, err := client.Subnetworks.List(cap.ProjectID, cap.Region).Do()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return encodeOutput(result)
}

func listServiceAccounts(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ProjectID == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("projectId is required")
	}

	client, err := getIamServiceClient(ctx, cap.Credentials)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	name := "projects/" + cap.ProjectID
	result, err := client.Projects.ServiceAccounts.List(name).Do()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return encodeOutput(result)
}

func listVersions(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ProjectID == "" || cap.Zone == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("projectId and zone are required")
	}

	client, err := getContainerServiceClient(ctx, cap.Credentials)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	result, err := client.Projects.Zones.GetServerconfig(cap.ProjectID, cap.Zone).Do()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return encodeOutput(result)
}

func listZones(ctx context.Context, cap *Capabilities) ([]byte, int, error) {
	if cap.ProjectID == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("projectId is required")
	}

	client, err := getComputeServiceClient(ctx, cap.Credentials)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	result, err := client.Zones.List(cap.ProjectID).Do()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return encodeOutput(result)
}

func encodeOutput(result interface{}) ([]byte, int, error) {
	data, err := json.Marshal(&result)
	if err != nil {
		return data, http.StatusInternalServerError, err
	}

	return data, http.StatusOK, err
}
