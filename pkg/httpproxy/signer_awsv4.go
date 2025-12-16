package httpproxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
)

const (
	defaultAWSRegion      = "us-east-1"
	defaultUSGovAWSRegion = "us-gov-west-1"
	cnNorth1AWSRegion     = "cn-north-1"
	cnNorthwest1AWSRegion = "cn-northwest-1"
)

// List of global services for AWS from: https://docs.aws.amazon.com/general/latest/gr/rande.html#global-endpoints
var globalAWSServices = []string{"cloudfront", "globalaccelerator", "iam", "networkmanager", "organizations", "route53", "shield", "waf"}

var requiredHeadersForAws = map[string]bool{"host": true,
	"x-amz-content-sha256": true,
	"x-amz-date":           true,
	"x-amz-user-agent":     true}

func (a awsv4) sign(req *http.Request, secrets SecretGetter, auth string) error {
	_, secret, err := getAuthData(auth, secrets, []string{"credID"})
	if err != nil {
		return err
	}
	service, region := a.getServiceAndRegion(req.URL.Host)
	credentialProvider := credentials.NewStaticCredentialsProvider(secret["accessKey"], secret["secretKey"], "")
	awsSigner := v4.NewSigner()
	var body []byte
	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("error reading request body %v", err)
		}
	}

	h := sha256.New()
	h.Write(body)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	oldHeader, newHeader := http.Header{}, http.Header{}
	for header, value := range req.Header {
		if _, ok := requiredHeadersForAws[strings.ToLower(header)]; ok {
			newHeader[header] = value
		} else {
			oldHeader[header] = value
		}
	}
	req.Header = newHeader
	err = awsSigner.SignHTTP(req.Context(), credentialProvider.Value, req, payloadHash, service, region, time.Now())
	if err != nil {
		return err
	}

	// The V2 SDK does not implement internally the sign with body method as per https://github.com/aws/aws-sdk-go/blob/main/aws/signer/v4/v4.go#L357
	// Therefore we need the below in order for the body to be included with the forwarded request.

	var reader io.ReadCloser
	var bodyReader io.ReadSeeker
	bodyReader = bytes.NewReader(body)
	if bodyReader != nil {
		var ok bool
		if reader, ok = bodyReader.(io.ReadCloser); !ok {
			reader = io.NopCloser(bodyReader)
		}
	}
	req.Body = reader

	for key, val := range oldHeader {
		req.Header.Add(key, strings.Join(val, ""))
	}
	return nil
}

func (a awsv4) getServiceAndRegion(host string) (string, string) {
	service := ""
	region := ""
	for _, partition := range endpoints.DefaultPartitions() {
		service, region = partitionServiceAndRegion(partition, host)
		// Some services are global and don't have a region. If a partition returns a service
		// that is global then stop processing partitions. If we carry on processing partitions
		// for a global service then when new partitions are introduced the signing may break.
		if service != "" && region == "" {
			if slices.Contains(globalAWSServices, service) {
				break
			}
		}

		// empty region is valid, but if one is found it should be assumed correct
		if region != "" {
			return service, region
		}
	}
	if strings.EqualFold(service, "iam") {
		// This conditional is meant to cover a discrepancy in the IAM service for the China regions.
		// The following doc states that IAM uses a globally unique endpoint, and the default
		// region "us-east-1" should be used as part of the Credential authentication parameter
		// (Current backend behavior). However, using "us-east-1" with any of the China regions will throw
		// the error "SignatureDoesNotMatch: Credential should be scoped to a valid region, not 'us-east-1'.".
		// https://docs.aws.amazon.com/general/latest/gr/sigv4_elements.html
		//
		// This other doc states the region value for China services should be "cn-north-1" or "cn-northwest-1"
		// including IAM (See IAM endpoints in the tables). So they need to be set manually to prevent the error
		// caused by the "us-east-1" default.
		// https://docs.amazonaws.cn/en_us/aws/latest/userguide/endpoints-Beijing.html
		if strings.Contains(host, cnNorth1AWSRegion) {
			return service, cnNorth1AWSRegion
		}
		if strings.Contains(host, cnNorthwest1AWSRegion) {
			return service, cnNorthwest1AWSRegion
		}
	}
	// if no region is found, global endpoint is assumed.
	// https://docs.aws.amazon.com/general/latest/gr/sigv4_elements.html
	if strings.Contains(host, "us-gov") {
		return service, defaultUSGovAWSRegion
	}

	return service, defaultAWSRegion
}

func partitionServiceAndRegion(partition endpoints.Partition, host string) (string, string) {
	service := ""
	partitionServices := partition.Services()
	for _, part := range strings.Split(host, ".") {
		if id := partitionServices[part].ID(); id != "" {
			service = id
			break
		}
	}

	if service == "" {
		return "", ""
	}

	host = strings.Trim(host, service)
	serviceRegions := partitionServices[service].Regions()
	for _, part := range strings.Split(host, ".") {
		if id := serviceRegions[part].ID(); id != "" {
			return service, id
		}
	}
	return service, ""
}
