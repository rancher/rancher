// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package core

import (
	"github.com/oracle/oci-go-sdk/common"
	"net/http"
)

// ListDedicatedVmHostsRequest wrapper for the ListDedicatedVmHosts operation
type ListDedicatedVmHostsRequest struct {

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the compartment.
	CompartmentId *string `mandatory:"true" contributesTo:"query" name:"compartmentId"`

	// The name of the availability domain.
	// Example: `Uocm:PHX-AD-1`
	AvailabilityDomain *string `mandatory:"false" contributesTo:"query" name:"availabilityDomain"`

	// A filter to only return resources that match the given lifecycle state.
	LifecycleState ListDedicatedVmHostsLifecycleStateEnum `mandatory:"false" contributesTo:"query" name:"lifecycleState" omitEmpty:"true"`

	// A filter to return only resources that match the given display name exactly.
	DisplayName *string `mandatory:"false" contributesTo:"query" name:"displayName"`

	// The name for the instance's shape.
	InstanceShapeName *string `mandatory:"false" contributesTo:"query" name:"instanceShapeName"`

	// For list pagination. The maximum number of results per page, or items to return in a paginated
	// "List" call. For important details about how pagination works, see
	// List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	// Example: `50`
	Limit *int `mandatory:"false" contributesTo:"query" name:"limit"`

	// For list pagination. The value of the `opc-next-page` response header from the previous "List"
	// call. For important details about how pagination works, see
	// List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	Page *string `mandatory:"false" contributesTo:"query" name:"page"`

	// Unique identifier for the request.
	// If you need to contact Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-request-id"`

	// The field to sort by. You can provide one sort order (`sortOrder`). Default order for
	// TIMECREATED is descending. Default order for DISPLAYNAME is ascending. The DISPLAYNAME
	// sort order is case sensitive.
	// **Note:** In general, some "List" operations (for example, `ListInstances`) let you
	// optionally filter by availability domain if the scope of the resource type is within a
	// single availability domain. If you call one of these "List" operations without specifying
	// an availability domain, the resources are grouped by availability domain, then sorted.
	SortBy ListDedicatedVmHostsSortByEnum `mandatory:"false" contributesTo:"query" name:"sortBy" omitEmpty:"true"`

	// The sort order to use, either ascending (`ASC`) or descending (`DESC`). The DISPLAYNAME sort order
	// is case sensitive.
	SortOrder ListDedicatedVmHostsSortOrderEnum `mandatory:"false" contributesTo:"query" name:"sortOrder" omitEmpty:"true"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request ListDedicatedVmHostsRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request ListDedicatedVmHostsRequest) HTTPRequest(method, path string) (http.Request, error) {
	return common.MakeDefaultHTTPRequestWithTaggedStruct(method, path, request)
}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request ListDedicatedVmHostsRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// ListDedicatedVmHostsResponse wrapper for the ListDedicatedVmHosts operation
type ListDedicatedVmHostsResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// A list of []DedicatedVmHostSummary instances
	Items []DedicatedVmHostSummary `presentIn:"body"`

	// For list pagination. When this header appears in the response, additional pages
	// of results remain. For important details about how pagination works, see
	// List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	OpcNextPage *string `presentIn:"header" name:"opc-next-page"`

	// Unique Oracle-assigned identifier for the request. If you need to contact
	// Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`
}

func (response ListDedicatedVmHostsResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response ListDedicatedVmHostsResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}

// ListDedicatedVmHostsLifecycleStateEnum Enum with underlying type: string
type ListDedicatedVmHostsLifecycleStateEnum string

// Set of constants representing the allowable values for ListDedicatedVmHostsLifecycleStateEnum
const (
	ListDedicatedVmHostsLifecycleStateCreating ListDedicatedVmHostsLifecycleStateEnum = "CREATING"
	ListDedicatedVmHostsLifecycleStateActive   ListDedicatedVmHostsLifecycleStateEnum = "ACTIVE"
	ListDedicatedVmHostsLifecycleStateUpdating ListDedicatedVmHostsLifecycleStateEnum = "UPDATING"
	ListDedicatedVmHostsLifecycleStateDeleting ListDedicatedVmHostsLifecycleStateEnum = "DELETING"
	ListDedicatedVmHostsLifecycleStateDeleted  ListDedicatedVmHostsLifecycleStateEnum = "DELETED"
	ListDedicatedVmHostsLifecycleStateFailed   ListDedicatedVmHostsLifecycleStateEnum = "FAILED"
)

var mappingListDedicatedVmHostsLifecycleState = map[string]ListDedicatedVmHostsLifecycleStateEnum{
	"CREATING": ListDedicatedVmHostsLifecycleStateCreating,
	"ACTIVE":   ListDedicatedVmHostsLifecycleStateActive,
	"UPDATING": ListDedicatedVmHostsLifecycleStateUpdating,
	"DELETING": ListDedicatedVmHostsLifecycleStateDeleting,
	"DELETED":  ListDedicatedVmHostsLifecycleStateDeleted,
	"FAILED":   ListDedicatedVmHostsLifecycleStateFailed,
}

// GetListDedicatedVmHostsLifecycleStateEnumValues Enumerates the set of values for ListDedicatedVmHostsLifecycleStateEnum
func GetListDedicatedVmHostsLifecycleStateEnumValues() []ListDedicatedVmHostsLifecycleStateEnum {
	values := make([]ListDedicatedVmHostsLifecycleStateEnum, 0)
	for _, v := range mappingListDedicatedVmHostsLifecycleState {
		values = append(values, v)
	}
	return values
}

// ListDedicatedVmHostsSortByEnum Enum with underlying type: string
type ListDedicatedVmHostsSortByEnum string

// Set of constants representing the allowable values for ListDedicatedVmHostsSortByEnum
const (
	ListDedicatedVmHostsSortByTimecreated ListDedicatedVmHostsSortByEnum = "TIMECREATED"
	ListDedicatedVmHostsSortByDisplayname ListDedicatedVmHostsSortByEnum = "DISPLAYNAME"
)

var mappingListDedicatedVmHostsSortBy = map[string]ListDedicatedVmHostsSortByEnum{
	"TIMECREATED": ListDedicatedVmHostsSortByTimecreated,
	"DISPLAYNAME": ListDedicatedVmHostsSortByDisplayname,
}

// GetListDedicatedVmHostsSortByEnumValues Enumerates the set of values for ListDedicatedVmHostsSortByEnum
func GetListDedicatedVmHostsSortByEnumValues() []ListDedicatedVmHostsSortByEnum {
	values := make([]ListDedicatedVmHostsSortByEnum, 0)
	for _, v := range mappingListDedicatedVmHostsSortBy {
		values = append(values, v)
	}
	return values
}

// ListDedicatedVmHostsSortOrderEnum Enum with underlying type: string
type ListDedicatedVmHostsSortOrderEnum string

// Set of constants representing the allowable values for ListDedicatedVmHostsSortOrderEnum
const (
	ListDedicatedVmHostsSortOrderAsc  ListDedicatedVmHostsSortOrderEnum = "ASC"
	ListDedicatedVmHostsSortOrderDesc ListDedicatedVmHostsSortOrderEnum = "DESC"
)

var mappingListDedicatedVmHostsSortOrder = map[string]ListDedicatedVmHostsSortOrderEnum{
	"ASC":  ListDedicatedVmHostsSortOrderAsc,
	"DESC": ListDedicatedVmHostsSortOrderDesc,
}

// GetListDedicatedVmHostsSortOrderEnumValues Enumerates the set of values for ListDedicatedVmHostsSortOrderEnum
func GetListDedicatedVmHostsSortOrderEnumValues() []ListDedicatedVmHostsSortOrderEnum {
	values := make([]ListDedicatedVmHostsSortOrderEnum, 0)
	for _, v := range mappingListDedicatedVmHostsSortOrder {
		values = append(values, v)
	}
	return values
}
