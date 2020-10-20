// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Identity and Access Management Service API
//
// APIs for managing users, groups, compartments, and policies.
//

package identity

import (
	"encoding/json"
	"github.com/oracle/oci-go-sdk/common"
)

// UpdateIdentityProviderDetails The representation of UpdateIdentityProviderDetails
type UpdateIdentityProviderDetails interface {

	// The description you assign to the `IdentityProvider`. Does not have to
	// be unique, and it's changeable.
	GetDescription() *string

	// Free-form tags for this resource. Each tag is a simple key-value pair with no predefined name, type, or namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	GetFreeformTags() map[string]string

	// Defined tags for this resource. Each key is predefined and scoped to a namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	GetDefinedTags() map[string]map[string]interface{}
}

type updateidentityproviderdetails struct {
	JsonData     []byte
	Description  *string                           `mandatory:"false" json:"description"`
	FreeformTags map[string]string                 `mandatory:"false" json:"freeformTags"`
	DefinedTags  map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`
	Protocol     string                            `json:"protocol"`
}

// UnmarshalJSON unmarshals json
func (m *updateidentityproviderdetails) UnmarshalJSON(data []byte) error {
	m.JsonData = data
	type Unmarshalerupdateidentityproviderdetails updateidentityproviderdetails
	s := struct {
		Model Unmarshalerupdateidentityproviderdetails
	}{}
	err := json.Unmarshal(data, &s.Model)
	if err != nil {
		return err
	}
	m.Description = s.Model.Description
	m.FreeformTags = s.Model.FreeformTags
	m.DefinedTags = s.Model.DefinedTags
	m.Protocol = s.Model.Protocol

	return err
}

// UnmarshalPolymorphicJSON unmarshals polymorphic json
func (m *updateidentityproviderdetails) UnmarshalPolymorphicJSON(data []byte) (interface{}, error) {

	if data == nil || string(data) == "null" {
		return nil, nil
	}

	var err error
	switch m.Protocol {
	case "SAML2":
		mm := UpdateSaml2IdentityProviderDetails{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	default:
		return *m, nil
	}
}

//GetDescription returns Description
func (m updateidentityproviderdetails) GetDescription() *string {
	return m.Description
}

//GetFreeformTags returns FreeformTags
func (m updateidentityproviderdetails) GetFreeformTags() map[string]string {
	return m.FreeformTags
}

//GetDefinedTags returns DefinedTags
func (m updateidentityproviderdetails) GetDefinedTags() map[string]map[string]interface{} {
	return m.DefinedTags
}

func (m updateidentityproviderdetails) String() string {
	return common.PointerString(m)
}

// UpdateIdentityProviderDetailsProtocolEnum Enum with underlying type: string
type UpdateIdentityProviderDetailsProtocolEnum string

// Set of constants representing the allowable values for UpdateIdentityProviderDetailsProtocolEnum
const (
	UpdateIdentityProviderDetailsProtocolSaml2 UpdateIdentityProviderDetailsProtocolEnum = "SAML2"
)

var mappingUpdateIdentityProviderDetailsProtocol = map[string]UpdateIdentityProviderDetailsProtocolEnum{
	"SAML2": UpdateIdentityProviderDetailsProtocolSaml2,
}

// GetUpdateIdentityProviderDetailsProtocolEnumValues Enumerates the set of values for UpdateIdentityProviderDetailsProtocolEnum
func GetUpdateIdentityProviderDetailsProtocolEnumValues() []UpdateIdentityProviderDetailsProtocolEnum {
	values := make([]UpdateIdentityProviderDetailsProtocolEnum, 0)
	for _, v := range mappingUpdateIdentityProviderDetailsProtocol {
		values = append(values, v)
	}
	return values
}
