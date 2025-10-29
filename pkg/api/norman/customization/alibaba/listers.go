package alibaba

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	cs "github.com/rancher/muchang/cs/client"
	ecs "github.com/rancher/muchang/ecs/client"
	resourcemanager "github.com/rancher/muchang/resourcemanager/client"
	vpc "github.com/rancher/muchang/vpc/client"
	"github.com/sirupsen/logrus"

	"github.com/rancher/muchang/utils/tea"
	"github.com/rancher/muchang/utils/tea/dara"
	"github.com/rancher/norman/httperror"
)

const (
	emptyResponseError = "received empty response"
)

const (
	defaultPageSize = 20
	defaultPageNum  = 1
)

func describeClusters(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateCSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)

	request := &cs.DescribeClustersForRegionRequest{
		PageSize:   &pageSize,
		PageNumber: &pageNumber,
	}

	clusterType := req.URL.Query().Get("clusterType")
	if clusterType != "" {
		request.ClusterType = &clusterType
	}

	resp, err := client.DescribeClustersForRegionWithContext(req.Context(), &capabilities.RegionID,
		request, map[string]*string{}, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeRegions(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	if capabilities.RegionID == "" {
		capabilities.RegionID = defaultRegion
	}

	client, err := CreateECSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &ecs.DescribeRegionsRequest{}
	acceptLanguage := req.URL.Query().Get("acceptLanguage")
	if acceptLanguage != "" {
		request.AcceptLanguage = &acceptLanguage
	}

	response, err := client.DescribeRegionsWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if response == nil || response.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(response.Body.String()), http.StatusOK, nil
}

func describeInstanceTypes(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateECSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &ecs.DescribeInstanceTypesRequest{}

	nextToken := req.URL.Query().Get("nextToken")
	if nextToken != "" {
		request.NextToken = &nextToken
	}

	maxResults := req.URL.Query().Get("maxResults")
	if maxResults != "" {
		maxResultsVal, err := strconv.ParseInt(maxResults, 10, 64)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("invalid value for maxResults query param")
		}

		request.MaxResults = &maxResultsVal
	}

	cpuArch := req.URL.Query().Get("cpuArch")
	if cpuArch != "" {
		request.CpuArchitecture = tea.String(cpuArch)
	}

	minCPUStr := req.URL.Query().Get("minCpuCores")
	if minCPUStr != "" {
		minCPU, err := strconv.ParseInt(minCPUStr, 10, 32)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("invalid value for minCpuCores query param")
		}
		minCPU32 := int32(minCPU)
		request.MinimumCpuCoreCount = &minCPU32
	}

	maxCPUStr := req.URL.Query().Get("maxCpuCores")
	if maxCPUStr != "" {
		maxCPU, err := strconv.ParseInt(maxCPUStr, 10, 32)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("invalid value for maxCpuCores query param")
		}
		maxCPU32 := int32(maxCPU)
		request.MaximumCpuCoreCount = &maxCPU32
	}

	minMemoryStr := req.URL.Query().Get("minMemorySize")
	if minMemoryStr != "" {
		minMemory, err := strconv.ParseFloat(minMemoryStr, 32)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("invalid value for minMemorySize query param")
		}
		minMemory32 := float32(minMemory)
		request.MinimumMemorySize = &minMemory32
	}

	maxMemoryStr := req.URL.Query().Get("maxMemorySize")
	if maxMemoryStr != "" {
		maxMemory, err := strconv.ParseFloat(maxMemoryStr, 32)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("invalid value for maxMemorySize query param")
		}
		maxMemory32 := float32(maxMemory)
		request.MaximumMemorySize = &maxMemory32
	}

	resp, err := client.DescribeInstanceTypesWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeKeyPairs(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateECSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)
	pSize, pNum := int32(pageSize), int32(pageNumber)

	request := &ecs.DescribeKeyPairsRequest{
		PageNumber: &pSize,
		PageSize:   &pNum,
	}

	resourceGroupID := req.URL.Query().Get("resourceGroupId")
	if resourceGroupID != "" {
		request.ResourceGroupId = &resourceGroupID
	}

	resp, err := client.DescribeKeyPairsWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeAvailableResource(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	// a required param: destinationResource
	destinationResource := req.URL.Query().Get("destinationResource")
	if destinationResource == "" {
		return nil, httperror.InvalidReference.Status, fmt.Errorf("invalid param destinationResource")
	}

	client, err := CreateECSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &ecs.DescribeAvailableResourceRequest{
		RegionId:            &capabilities.RegionID,
		DestinationResource: &destinationResource,
	}

	networkCategory := req.URL.Query().Get("networkCategory")
	if networkCategory != "" {
		request.NetworkCategory = &networkCategory
	}
	zoneID := req.URL.Query().Get("zoneId")
	if zoneID != "" {
		request.ZoneId = &zoneID
	}
	resourceType := req.URL.Query().Get("resourceType")
	if resourceType != "" {
		request.ResourceType = &resourceType
	}
	instanceType := req.URL.Query().Get("instanceType")
	if instanceType != "" {
		request.InstanceType = &instanceType
	}

	resp, err := client.DescribeAvailableResourceWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeResourceGroups(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateResourceManagerClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)
	pSize, pNum := int32(pageSize), int32(pageNumber)

	listResourceGroupsRequest := &resourcemanager.ListResourceGroupsRequest{
		PageSize:   &pSize,
		PageNumber: &pNum,
	}

	resp, err := client.ListResourceGroups(listResourceGroupsRequest)
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeVpcs(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateVpcClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)
	pSize, pNum := int32(pageSize), int32(pageNumber)

	request := &vpc.DescribeVpcsRequest{
		PageSize:   &pSize,
		PageNumber: &pNum,
	}

	resourceGroupID := req.URL.Query().Get("resourceGroupId")
	if resourceGroupID != "" {
		request.ResourceGroupId = &resourceGroupID
	}

	resp, err := client.DescribeVpcsWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeVSwitches(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateVpcClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)
	pSize, pNum := int32(pageSize), int32(pageNumber)

	request := &vpc.DescribeVSwitchesRequest{
		PageNumber: &pNum,
		PageSize:   &pSize,
	}

	vpcID := req.URL.Query().Get("vpcId")
	if vpcID != "" {
		request.VpcId = &vpcID
	}
	zoneID := req.URL.Query().Get("zoneId")
	if zoneID != "" {
		request.ZoneId = &zoneID
	}

	resourceGroupID := req.URL.Query().Get("resourceGroupId")
	if resourceGroupID != "" {
		request.ResourceGroupId = &resourceGroupID
	}

	resp, err := client.DescribeVSwitchesWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func getPageParams(req *http.Request, defaultSize, defaultNumber int64) (int64, int64) {
	query := req.URL.Query()

	pageSize := defaultSize
	pageNumber := defaultNumber

	if val := query.Get("pageSize"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil && parsed > 0 {
			pageSize = int64(parsed)
		}
	}

	if val := query.Get("pageNumber"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil && parsed > 0 {
			pageNumber = int64(parsed)
		}
	}

	return pageSize, pageNumber
}

func handleSDKError(err error) (int, error) {
	sdkErr, ok := err.(*tea.SDKError)
	if ok {
		errStatus := tea.IntValue(sdkErr.StatusCode)
		if errStatus == 0 {
			errStatus = httperror.ServerError.Status
		}
		if strings.Contains(tea.StringValue(sdkErr.Code), "InvalidAccessKeyId") || strings.Contains(tea.StringValue(sdkErr.Code), "SignatureDoesNotMatch") {
			return http.StatusUnauthorized, errors.New("invalid credentials")
		}

		return errStatus, errors.New(sdkErr.Error())
	}

	return httperror.ServerError.Status, err
}

func describeKubernetesMetadata(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateCSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &cs.DescribeKubernetesVersionMetadataRequest{
		Region: &capabilities.RegionID,
	}

	clusterType := req.URL.Query().Get("clusterType")
	if clusterType != "" {
		request.ClusterType = &clusterType
	}

	mode := req.URL.Query().Get("mode")
	if mode != "" {
		request.Mode = &mode
	}

	kubernetesVersion := req.URL.Query().Get("kubernetesVersion")
	if kubernetesVersion != "" {
		request.KubernetesVersion = &kubernetesVersion
	}

	getUpgradableVersions := req.URL.Query().Get("getUpgradableVersions")
	if getUpgradableVersions != "" {
		getUpgradableVersionsVal, err := strconv.ParseBool(getUpgradableVersions)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("getUpgradableVersions param value not valid")
		}
		request.QueryUpgradableVersion = tea.Bool(getUpgradableVersionsVal)
	}

	resp, err := client.DescribeKubernetesVersionMetadataWithContext(req.Context(), request, map[string]*string{}, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	bytes, err := json.Marshal(resp.Body)
	if err != nil {
		logrus.Debugf("[alibaba-handler] error parsing describeKubernetesVersionMetadata: %v", err)
		return nil, httperror.ServerError.Status, errors.New("error parsing response")
	}

	return bytes, http.StatusOK, nil
}

func describeZones(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateECSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &ecs.DescribeZonesRequest{
		RegionId: &capabilities.RegionID,
	}
	acceptLanguage := req.URL.Query().Get("acceptLanguage")
	if acceptLanguage != "" {
		request.AcceptLanguage = &acceptLanguage
	}

	response, err := client.DescribeZonesWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if response == nil || response.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(response.Body.String()), http.StatusOK, nil
}

func describeImageSupportedInstanceTypes(capabilities *Capabilities, req *http.Request) ([]byte, int, error) {
	client, err := CreateECSClient(capabilities.AccessKeyID, capabilities.AccessKeySecret, capabilities.RegionID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &ecs.DescribeImageSupportInstanceTypesRequest{
		RegionId: &capabilities.RegionID,
	}

	imageID := req.URL.Query().Get("imageId")
	if imageID != "" {
		request.ImageId = &imageID
	}

	action := req.URL.Query().Get("action")
	if action != "" {
		request.ActionType = &action
	}

	resp, err := client.DescribeImageSupportInstanceTypesWithContext(req.Context(), request, &dara.RuntimeOptions{})
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}
