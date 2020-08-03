// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package core

import (
	"github.com/oracle/oci-go-sdk/common"
	"net/http"
)

// CreateDedicatedVmHostRequest wrapper for the CreateDedicatedVmHost operation
type CreateDedicatedVmHostRequest struct {

	// The details for creating a new dedicated virtual machine host.
	CreateDedicatedVmHostDetails `contributesTo:"body"`

	// Unique identifier for the request.
	// If you need to contact Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-request-id"`

	// A token that uniquely identifies a request so it can be retried in case of a timeout or
	// server error without risk of executing that same action again. Retry tokens expire after 24
	// hours, but can be invalidated before then due to conflicting operations (for example, if a resource
	// has been deleted and purged from the system, then a retry of the original creation request
	// may be rejected).
	OpcRetryToken *string `mandatory:"false" contributesTo:"header" name:"opc-retry-token"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request CreateDedicatedVmHostRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request CreateDedicatedVmHostRequest) HTTPRequest(method, path string) (http.Request, error) {
	return common.MakeDefaultHTTPRequestWithTaggedStruct(method, path, request)
}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request CreateDedicatedVmHostRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// CreateDedicatedVmHostResponse wrapper for the CreateDedicatedVmHost operation
type CreateDedicatedVmHostResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// The DedicatedVmHost instance
	DedicatedVmHost `presentIn:"body"`

	// For optimistic concurrency control. See `if-match`.
	Etag *string `presentIn:"header" name:"etag"`

	// Unique Oracle-assigned identifier for the request. If you need to contact
	// Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`

	// The OCID of the work request. Use GetWorkRequest (https://docs.cloud.oracle.com/api/#/en/workrequests/20160918/WorkRequest/GetWorkRequest)
	// with this ID to track the status of the request.
	OpcWorkRequestId *string `presentIn:"header" name:"opc-work-request-id"`
}

func (response CreateDedicatedVmHostResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response CreateDedicatedVmHostResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}
