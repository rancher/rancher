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

// OAuth2ClientCredentialSummary User can define Oauth clients in IAM, then use it to generate a token to grant access to app resources.
type OAuth2ClientCredentialSummary struct {

	// Allowed scopes for the given oauth credential.
	Scopes []FullyQualifiedScope `mandatory:"false" json:"scopes"`

	// The OCID of the user the Oauth credential belongs to.
	UserId *string `mandatory:"false" json:"userId"`

	// Date and time when this credential will expire, in the format defined by RFC3339.
	// Null if it never expires.
	// Example: `2016-08-25T21:10:29.600Z`
	ExpiresOn *common.SDKTime `mandatory:"false" json:"expiresOn"`

	// The OCID of the Oauth credential.
	Id *string `mandatory:"false" json:"id"`

	// The OCID of the compartment containing the Oauth credential.
	CompartmentId *string `mandatory:"false" json:"compartmentId"`

	// The name of the Oauth credential.
	Name *string `mandatory:"false" json:"name"`

	// The description of the Oauth credential.
	Description *string `mandatory:"false" json:"description"`

	// The credential's current state. After creating a Oauth credential, make sure its `lifecycleState` changes from
	// CREATING to ACTIVE before using it.
	LifecycleState OAuth2ClientCredentialSummaryLifecycleStateEnum `mandatory:"false" json:"lifecycleState,omitempty"`

	// Date and time the `OAuth2ClientCredential` object was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`
}

func (m OAuth2ClientCredentialSummary) String() string {
	return common.PointerString(m)
}

// OAuth2ClientCredentialSummaryLifecycleStateEnum Enum with underlying type: string
type OAuth2ClientCredentialSummaryLifecycleStateEnum string

// Set of constants representing the allowable values for OAuth2ClientCredentialSummaryLifecycleStateEnum
const (
	OAuth2ClientCredentialSummaryLifecycleStateCreating OAuth2ClientCredentialSummaryLifecycleStateEnum = "CREATING"
	OAuth2ClientCredentialSummaryLifecycleStateActive   OAuth2ClientCredentialSummaryLifecycleStateEnum = "ACTIVE"
	OAuth2ClientCredentialSummaryLifecycleStateInactive OAuth2ClientCredentialSummaryLifecycleStateEnum = "INACTIVE"
	OAuth2ClientCredentialSummaryLifecycleStateDeleting OAuth2ClientCredentialSummaryLifecycleStateEnum = "DELETING"
	OAuth2ClientCredentialSummaryLifecycleStateDeleted  OAuth2ClientCredentialSummaryLifecycleStateEnum = "DELETED"
)

var mappingOAuth2ClientCredentialSummaryLifecycleState = map[string]OAuth2ClientCredentialSummaryLifecycleStateEnum{
	"CREATING": OAuth2ClientCredentialSummaryLifecycleStateCreating,
	"ACTIVE":   OAuth2ClientCredentialSummaryLifecycleStateActive,
	"INACTIVE": OAuth2ClientCredentialSummaryLifecycleStateInactive,
	"DELETING": OAuth2ClientCredentialSummaryLifecycleStateDeleting,
	"DELETED":  OAuth2ClientCredentialSummaryLifecycleStateDeleted,
}

// GetOAuth2ClientCredentialSummaryLifecycleStateEnumValues Enumerates the set of values for OAuth2ClientCredentialSummaryLifecycleStateEnum
func GetOAuth2ClientCredentialSummaryLifecycleStateEnumValues() []OAuth2ClientCredentialSummaryLifecycleStateEnum {
	values := make([]OAuth2ClientCredentialSummaryLifecycleStateEnum, 0)
	for _, v := range mappingOAuth2ClientCredentialSummaryLifecycleState {
		values = append(values, v)
	}
	return values
}
