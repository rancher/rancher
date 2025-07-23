package alibaba

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	cs "github.com/alibabacloud-go/cs-20151215/v5/client"
	ecs "github.com/alibabacloud-go/ecs-20140526/v7/client"
	resourcemanager "github.com/alibabacloud-go/resourcemanager-20200331/v3/client"
	vpc "github.com/alibabacloud-go/vpc-20160428/v6/client"
	"github.com/sirupsen/logrus"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/rancher/norman/httperror"
)

const (
	emptyResponseError = "received empty response"
)

const (
	defaultPageSize = 20
	defaultPageNum  = 1
)

func describeClusters(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	client, err := CreateCSClient(ak, sk, regionId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)

	request := &cs.DescribeClustersForRegionRequest{
		PageSize:   &pageSize,
		PageNumber: &pageNumber,
	}

	resp, err := client.DescribeClustersForRegion(&regionId, request)
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeRegions(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	if regionId == "" {
		regionId = defaultRegion
	}

	client, err := CreateECSClient(ak, sk, regionId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &ecs.DescribeRegionsRequest{}
	acceptLanguage := req.URL.Query().Get("acceptLanguage")
	if acceptLanguage != "" {
		request.AcceptLanguage = &acceptLanguage
	}

	response, err := client.DescribeRegions(request)
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if response == nil || response.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(response.Body.String()), http.StatusOK, nil
}

func describeInstanceTypes(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	client, err := CreateECSClient(ak, sk, regionId)
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

	resp, err := client.DescribeInstanceTypes(request)
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeKeyPairs(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	client, err := CreateECSClient(ak, sk, regionId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)
	pSize, pNum := int32(pageSize), int32(pageNumber)

	request := &ecs.DescribeKeyPairsRequest{
		PageNumber: &pSize,
		PageSize:   &pNum,
	}

	resourceGroupId := req.URL.Query().Get("resourceGroupId")
	if resourceGroupId != "" {
		request.ResourceGroupId = &resourceGroupId
	}

	resp, err := client.DescribeKeyPairs(request)
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeAvailableResource(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	// a required param: destinationResource
	destinationResource := req.URL.Query().Get("destinationResource")
	if destinationResource == "" {
		return nil, httperror.InvalidReference.Status, fmt.Errorf("invalid param destinationResource")
	}

	client, err := CreateECSClient(ak, sk, regionId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &ecs.DescribeAvailableResourceRequest{
		RegionId:            &regionId,
		DestinationResource: &destinationResource,
	}

	networkCategory := req.URL.Query().Get("networkCategory")
	if networkCategory != "" {
		request.NetworkCategory = &networkCategory
	}
	zoneId := req.URL.Query().Get("zoneId")
	if zoneId != "" {
		request.ZoneId = &zoneId
	}

	resp, err := client.DescribeAvailableResource(request)
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.AvailableZones.String()), http.StatusOK, nil
}

func describeResourceGroups(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	client, err := CreateResourceManagerClient(ak, sk, regionId)
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

func describeVpcs(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	client, err := CreateVpcClient(ak, sk, regionId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)
	pSize, pNum := int32(pageSize), int32(pageNumber)

	request := &vpc.DescribeVpcsRequest{
		PageSize:   &pSize,
		PageNumber: &pNum,
	}

	resourceGroupId := req.URL.Query().Get("resourceGroupId")
	if resourceGroupId != "" {
		request.ResourceGroupId = &resourceGroupId
	}

	resp, err := client.DescribeVpcs(request)
	if err != nil {
		status, err := handleSDKError(err)
		return nil, status, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, errors.New(emptyResponseError)
	}

	return []byte(resp.Body.String()), http.StatusOK, nil
}

func describeVSwitches(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	client, err := CreateVpcClient(ak, sk, regionId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	pageSize, pageNumber := getPageParams(req, defaultPageSize, defaultPageNum)
	pSize, pNum := int32(pageSize), int32(pageNumber)

	request := &vpc.DescribeVSwitchesRequest{
		PageNumber: &pNum,
		PageSize:   &pSize,
	}

	vpcId := req.URL.Query().Get("vpcId")
	if vpcId != "" {
		request.VpcId = &vpcId
	}
	zoneId := req.URL.Query().Get("zoneId")
	if zoneId != "" {
		request.ZoneId = &zoneId
	}

	resourceGroupId := req.URL.Query().Get("resourceGroupId")
	if resourceGroupId != "" {
		request.ResourceGroupId = &resourceGroupId
	}

	resp, err := client.DescribeVSwitches(request)
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
			errStatus = http.StatusUnauthorized
		}

		return errStatus, errors.New(sdkErr.Error())
	}

	return httperror.ServerError.Status, err
}

func describeKubernetesMetadata(ak, sk, regionId string, req *http.Request) ([]byte, int, error) {
	client, err := CreateCSClient(ak, sk, regionId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	request := &cs.DescribeKubernetesVersionMetadataRequest{
		Region: &regionId,
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

	resp, err := client.DescribeKubernetesVersionMetadata(request)
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
