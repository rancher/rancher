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

// UpdateUserCapabilitiesDetails The representation of UpdateUserCapabilitiesDetails
type UpdateUserCapabilitiesDetails struct {

	// Indicates if the user can log in to the console.
	CanUseConsolePassword *bool `mandatory:"false" json:"canUseConsolePassword"`

	// Indicates if the user can use API keys.
	CanUseApiKeys *bool `mandatory:"false" json:"canUseApiKeys"`

	// Indicates if the user can use SWIFT passwords / auth tokens.
	CanUseAuthTokens *bool `mandatory:"false" json:"canUseAuthTokens"`

	// Indicates if the user can use SMTP passwords.
	CanUseSmtpCredentials *bool `mandatory:"false" json:"canUseSmtpCredentials"`

	// Indicates if the user can use SigV4 symmetric keys.
	CanUseCustomerSecretKeys *bool `mandatory:"false" json:"canUseCustomerSecretKeys"`

	// Indicates if the user can use OAuth2 credentials and tokens.
	CanUseOAuth2ClientCredentials *bool `mandatory:"false" json:"canUseOAuth2ClientCredentials"`
}

func (m UpdateUserCapabilitiesDetails) String() string {
	return common.PointerString(m)
}
