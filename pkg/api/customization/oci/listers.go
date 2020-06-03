package oci

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/containerengine"
	"github.com/oracle/oci-go-sdk/core"
	"github.com/oracle/oci-go-sdk/identity"
	"github.com/rancher/norman/httperror"
	"github.com/sirupsen/logrus"
)

func processVcns(provider common.ConfigurationProvider, compartment string) ([]byte, int, error) {
	logrus.Debugf("[oci-handler] listing VCNs in compartment: %s", compartment)
	virtualNetworkClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating Virtual Network client: %v", err)
		return nil, httperror.ServerError.Status, err
	}
	vcnRequest := core.ListVcnsRequest{
		CompartmentId: &compartment,
	}
	vcnResponse, err := virtualNetworkClient.ListVcns(context.Background(), vcnRequest)
	if err != nil {
		httpErr := httperror.ErrorCode{}
		if vcnResponse.RawResponse != nil {
			httpErr.Status = vcnResponse.RawResponse.StatusCode
		} else {
			httpErr.Status = httperror.ServerError.Status
		}
		logrus.Debugf("[oci-handler] error listing VCNs with Virtual Network client: %v", err)
		return nil, httpErr.Status, err
	}

	var vcnDisplayNames []string
	for _, item := range vcnResponse.Items {
		vcnDisplayNames = append(vcnDisplayNames, *(item.DisplayName))
	}

	data, err := json.Marshal(vcnDisplayNames)
	if err != nil {
		return data, httperror.ServerError.Status, err
	}

	return data, http.StatusOK, err
}

func processOkeVersions(provider common.ConfigurationProvider) ([]byte, int, error) {
	logrus.Debug("[oci-handler] listing OKE versions")
	containerClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating ContainerEngine client: %v", err)
		return nil, httperror.ServerError.Status, err
	}
	getClusterOptionsReq := containerengine.GetClusterOptionsRequest{
		ClusterOptionId: common.String("all"),
	}
	getClusterOptionsResp, err := containerClient.GetClusterOptions(context.Background(), getClusterOptionsReq)
	if err != nil {
		httpErr := httperror.ErrorCode{}
		if getClusterOptionsResp.RawResponse != nil {
			httpErr.Status = getClusterOptionsResp.RawResponse.StatusCode
		} else {
			httpErr.Status = httperror.ServerError.Status
		}
		logrus.Debugf("[oci-handler] error getting cluster options with ContainerEngine client: %v", err)
		return nil, httpErr.Status, err
	}

	data, err := json.Marshal(getClusterOptionsResp.KubernetesVersions)
	if err != nil {
		return data, httperror.ServerError.Status, err
	}

	return data, http.StatusOK, err
}

func processAvailabilityDomains(provider common.ConfigurationProvider, compartment string) ([]byte, int, error) {
	logrus.Debugf("[oci-handler] listing ADs in compartment: %s", compartment)
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating Identity client: %v", err)
		return nil, httperror.ServerError.Status, err
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
	if err != nil {
		return data, httperror.ServerError.Status, err
	}

	return data, http.StatusOK, err
}

func processRegions(provider common.ConfigurationProvider, tenancy string) ([]byte, int, error) {
	logrus.Debugf("[oci-handler] listing VCNs in tenancy: %s", tenancy)
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating Identity client: %v", err)
		return nil, httperror.ServerError.Status, err
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
	if err != nil {
		return data, httperror.ServerError.Status, err
	}

	return data, http.StatusOK, err
}

func processNodeShapes(provider common.ConfigurationProvider, compartment string) ([]byte, int, error) {
	logrus.Debugf("[oci-handler] listing shapes in compartment: %s", compartment)
	computeClient, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating compute client: %v", err)
		return nil, httperror.ServerError.Status, err
	}
	shapeRequest := core.ListShapesRequest{
		CompartmentId: &compartment,
	}
	shapeResponse, err := computeClient.ListShapes(context.Background(), shapeRequest)
	if err != nil {
		logrus.Debugf("[oci-handler] error listing shapes with Compute client: %v", err)
		return nil, getErrorCode(shapeResponse.RawResponse), err
	}

	var nodeShapes []string
	for _, item := range shapeResponse.Items {
		if !listContains(nodeShapes, *(item.Shape)) {
			nodeShapes = append(nodeShapes, *(item.Shape))
		}
	}

	data, err := json.Marshal(nodeShapes)
	if err != nil {
		return data, httperror.ServerError.Status, err
	}

	return data, http.StatusOK, err
}

func processImages(provider common.ConfigurationProvider, compartment string) ([]byte, int, error) {
	logrus.Debugf("[oci-handler] listing images in compartment: %s", compartment)
	computeClient, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating Compute client: %v", err)
		return nil, httperror.ServerError.Status, err
	}
	imageRequest := core.ListImagesRequest{
		CompartmentId: &compartment,
	}
	shapeResponse, err := computeClient.ListImages(context.Background(), imageRequest)
	if err != nil {
		logrus.Debugf("[oci-handler] error listing images with Compute client: %v", err)
		return nil, getErrorCode(shapeResponse.RawResponse), err
	}

	var nodeImages []string
	for _, item := range shapeResponse.Items {
		if !strings.Contains(*item.DisplayName, "GPU") &&
			!strings.Contains(*item.DisplayName, "Oracle-Linux-6") &&
			strings.Contains(*item.DisplayName, "Oracle-Linux") &&
			!listContains(nodeImages, *(item.DisplayName)) {
			nodeImages = append(nodeImages, *(item.DisplayName))
		}
	}

	data, err := json.Marshal(nodeImages)
	if err != nil {
		return data, httperror.ServerError.Status, err
	}

	return data, http.StatusOK, err
}

func processNodeOkeImages(provider common.ConfigurationProvider) ([]byte, int, error) {
	logrus.Debugf("[oci-handler] listing node OKE images")
	containerClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(provider)
	if err != nil {
		logrus.Debugf("[oci-handler] error creating ContainerEngine client: %v", err)
		return nil, httperror.ServerError.Status, err
	}
	nodePoolOptionsReq := containerengine.GetNodePoolOptionsRequest{
		NodePoolOptionId: common.String("all"),
	}
	nodePoolOptionsResp, err := containerClient.GetNodePoolOptions(context.Background(), nodePoolOptionsReq)
	if err != nil {
		logrus.Debugf("[oci-handler] error getting node pool options with Compute client: %v", err)
		return nil, getErrorCode(nodePoolOptionsResp.RawResponse), err
	}

	var nodeSources []string
	for _, item := range nodePoolOptionsResp.Sources {

		sourceName := *(item.GetSourceName())

		if !listContains(nodeSources, sourceName) {
			nodeSources = append(nodeSources, sourceName)
		}
	}

	data, err := json.Marshal(nodeSources)
	if err != nil {
		return data, httperror.ServerError.Status, err
	}

	return data, http.StatusOK, err
}

func listContains(list []string, entry string) bool {
	for _, x := range list {
		if x == entry {
			return true
		}
	}
	return false
}

func getErrorCode(response *http.Response) int {
	if response == nil {
		return httperror.ServerError.Status
	}
	return response.StatusCode
}
