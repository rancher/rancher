package oci

import (
	"context"
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"

	"github.com/sirupsen/logrus"

	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/containerengine"
	"github.com/oracle/oci-go-sdk/core"
	"github.com/oracle/oci-go-sdk/identity"

	"encoding/json"
)

func processVcns(provider common.ConfigurationProvider, compartment string) ([]byte, httperror.ErrorCode, error) {
	logrus.Debugf("[oci-handler] listing VCNs in compartment: %s", compartment)
	virtualNetworkClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating OCI Virtual Network client: %v", err)
		return nil, httperror.ErrorCode{}, err
	}
	vcnRequest := core.ListVcnsRequest{
		CompartmentId: &compartment,
	}
	vcnResponse, err := virtualNetworkClient.ListVcns(context.Background(), vcnRequest)
	if err != nil {
		httpErr := httperror.ErrorCode{}
		if vcnResponse.RawResponse != nil {
			httpErr.Status = vcnResponse.RawResponse.StatusCode
			httpErr.Code = vcnResponse.RawResponse.Status
		} else {
			httpErr = httperror.ErrorCode{}
		}
		logrus.Debugf("[oci-handler] error listing VCNs with OCI Virtual Network client: %v", err)
		return nil, httpErr, err
	}

	var vcnDisplayNames []string
	for _, item := range vcnResponse.Items {
		vcnDisplayNames = append(vcnDisplayNames, *(item.DisplayName))
	}

	data, err := json.Marshal(vcnDisplayNames)

	return data, httperror.ErrorCode{}, err
}

func processOkeVersions(provider common.ConfigurationProvider) ([]byte, httperror.ErrorCode, error) {
	logrus.Debug("[oci-handler] listing OKE versions")
	containerClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating OCI ContainerEngine client: %v", err)
		return nil, httperror.ErrorCode{}, err
	}
	getClusterOptionsReq := containerengine.GetClusterOptionsRequest{
		ClusterOptionId: common.String("all"),
	}
	getClusterOptionsResp, err := containerClient.GetClusterOptions(context.Background(), getClusterOptionsReq)
	if err != nil {
		httpErr := httperror.ErrorCode{}
		if getClusterOptionsResp.RawResponse != nil {
			httpErr.Status = getClusterOptionsResp.RawResponse.StatusCode
			httpErr.Code = getClusterOptionsResp.RawResponse.Status
		} else {
			httpErr = httperror.ErrorCode{}
		}
		logrus.Debugf("[oci-handler] error getting cluster options with OCI ContainerEngine client: %v", err)
		return nil, httpErr, err
	}

	data, err := json.Marshal(getClusterOptionsResp.KubernetesVersions)

	return data, httperror.ErrorCode{}, err
}

func processAvailabilityDomains(provider common.ConfigurationProvider, compartment string) ([]byte, httperror.ErrorCode, error) {
	logrus.Debugf("[oci-handler] listing ADs in compartment: %s", compartment)
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating OCI Identity client: %v", err)
		return nil, httperror.ErrorCode{}, err
	}
	request := identity.ListAvailabilityDomainsRequest{
		CompartmentId: &compartment,
	}
	availabilityDomains, err := identityClient.ListAvailabilityDomains(context.Background(), request)
	if err != nil {
		logrus.Debugf("[oci-handler] error listing ADs with Identity client: %v", err)
		return nil, getErrorCode(availabilityDomains.RawResponse), err
	}

	var adNames []string
	for _, item := range availabilityDomains.Items {
		adNames = append(adNames, *(item.Name))
	}

	data, err := json.Marshal(adNames)

	return data, httperror.ErrorCode{}, err
}

func processRegions(provider common.ConfigurationProvider, tenancy string) ([]byte, httperror.ErrorCode, error) {
	logrus.Debugf("[oci-handler] listing VCNs in tenancy: %s", tenancy)
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating OCI Identity client: %v", err)
		return nil, httperror.ErrorCode{}, err
	}
	request := identity.ListRegionSubscriptionsRequest{
		TenancyId: &tenancy,
	}
	allRegions, err := identityClient.ListRegionSubscriptions(context.Background(), request)
	if err != nil {
		logrus.Debugf("[oci-handler] error listing regions with Identity client: %v", err)
		return nil, getErrorCode(allRegions.RawResponse), err
	}

	var regionNames []string
	for _, item := range allRegions.Items {
		regionNames = append(regionNames, *(item.RegionName))
	}

	data, err := json.Marshal(regionNames)

	return data, httperror.ErrorCode{}, err
}

func processNodeShapes(provider common.ConfigurationProvider, compartment string) ([]byte, httperror.ErrorCode, error) {
	logrus.Debugf("[oci-handler] listing shapes in compartment: %s", compartment)
	computeClient, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating OCI compute client: %v", err)
		return nil, httperror.ErrorCode{}, err
	}
	shapeRequest := core.ListShapesRequest{
		CompartmentId: &compartment,
	}
	shapeResponse, err := computeClient.ListShapes(context.Background(), shapeRequest)
	if err != nil {
		logrus.Debugf("[oci-handler] error listing shapes with OCI Compute client: %v", err)
		return nil, getErrorCode(shapeResponse.RawResponse), err
	}

	var nodeShapes []string
	for _, item := range shapeResponse.Items {
		if listContains(nodeShapes, *(item.Shape)) == false {
			nodeShapes = append(nodeShapes, *(item.Shape))
		}
	}

	data, err := json.Marshal(nodeShapes)

	return data, httperror.ErrorCode{}, err
}

func processImages(provider common.ConfigurationProvider, compartment string) ([]byte, httperror.ErrorCode, error) {
	logrus.Debugf("[oci-handler] listing images in compartment: %s", compartment)
	computeClient, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating OCI Compute client: %v", err)
		return nil, httperror.ErrorCode{}, err
	}
	imageRequest := core.ListImagesRequest{
		CompartmentId: &compartment,
	}
	shapeResponse, err := computeClient.ListImages(context.Background(), imageRequest)
	if err != nil {
		logrus.Debugf("[oci-handler] error listing images with OCI Compute client: %v", err)
		return nil, getErrorCode(shapeResponse.RawResponse), err
	}

	var nodeImages []string
	for _, item := range shapeResponse.Items {
		if !strings.Contains(*item.DisplayName, "GPU") &&
			!strings.Contains(*item.DisplayName, "Oracle-Linux-6") &&
			strings.Contains(*item.DisplayName, "Oracle-Linux") &&
			listContains(nodeImages, *(item.DisplayName)) == false {
			nodeImages = append(nodeImages, *(item.DisplayName))
		}
	}

	data, err := json.Marshal(nodeImages)

	return data, httperror.ErrorCode{}, err
}

func processNodeOkeImages(provider common.ConfigurationProvider) ([]byte, httperror.ErrorCode, error) {
	logrus.Debugf("[oci-handler] listing node OKE images")
	containerClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating OCI ContainerEngine client: %v", err)
		return nil, httperror.ErrorCode{}, err
	}
	nodePoolOptionsReq := containerengine.GetNodePoolOptionsRequest{
		NodePoolOptionId: common.String("all"),
	}
	nodePoolOptionsResp, err := containerClient.GetNodePoolOptions(context.Background(), nodePoolOptionsReq)
	if err != nil {
		logrus.Debugf("[oci-handler] error getting node pool options with OCI Compute client: %v", err)
		return nil, getErrorCode(nodePoolOptionsResp.RawResponse), err
	}

	var nodeSources []string
	for _, item := range nodePoolOptionsResp.Sources {

		sourceName := *(item.GetSourceName())

		if listContains(nodeSources, sourceName) == false {
			nodeSources = append(nodeSources, sourceName)
		}
	}

	data, err := json.Marshal(nodeSources)

	return data, httperror.ErrorCode{}, err
}

func listContains(list []string, entry string) bool {
	for _, x := range list {
		if x == entry {
			return true
		}
	}
	return false
}

func getErrorCode(response *http.Response) httperror.ErrorCode {
	if response == nil {
		return httperror.ServerError
	}
	return httperror.ErrorCode{Code: response.Status,
		Status: response.StatusCode}
}
