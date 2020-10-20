// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Core Services API
//
// API covering the Networking (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/overview.htm),
// Compute (https://docs.cloud.oracle.com/iaas/Content/Compute/Concepts/computeoverview.htm), and
// Block Volume (https://docs.cloud.oracle.com/iaas/Content/Block/Concepts/overview.htm) services. Use this API
// to manage resources such as virtual cloud networks (VCNs), compute instances, and
// block storage volumes.
//

package core

import (
	"github.com/oracle/oci-go-sdk/common"
)

// AppCatalogListing Listing details.
type AppCatalogListing struct {

	// Listing's contact URL.
	ContactUrl *string `mandatory:"false" json:"contactUrl"`

	// Description of the listing.
	Description *string `mandatory:"false" json:"description"`

	// The OCID of the listing.
	ListingId *string `mandatory:"false" json:"listingId"`

	// Name of the listing.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Date and time the listing was published, in RFC3339 format.
	// Example: `2018-03-20T12:32:53.532Z`
	TimePublished *common.SDKTime `mandatory:"false" json:"timePublished"`

	// Publisher's logo URL.
	PublisherLogoUrl *string `mandatory:"false" json:"publisherLogoUrl"`

	// Name of the publisher who published this listing.
	PublisherName *string `mandatory:"false" json:"publisherName"`

	// Summary of the listing.
	Summary *string `mandatory:"false" json:"summary"`
}

func (m AppCatalogListing) String() string {
	return common.PointerString(m)
}
