// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Identity and Access Management Service API
//
// APIs for managing users, groups, compartments, and policies.
//

package identity

import (
	"github.com/oracle/oci-go-sdk/common"
)

// UpdateOAuth2ClientCredentialDetails The representation of UpdateOAuth2ClientCredentialDetails
type UpdateOAuth2ClientCredentialDetails struct {

	// Description of the oauth credential to help user differentiate them.
	Description *string `mandatory:"true" json:"description"`

	// Allowed scopes for the given oauth credential.
	Scopes []FullyQualifiedScope `mandatory:"true" json:"scopes"`

	// Indicate if the password to be reset or not in the update.
	IsResetPassword *bool `mandatory:"false" json:"isResetPassword"`
}

func (m UpdateOAuth2ClientCredentialDetails) String() string {
	return common.PointerString(m)
}
