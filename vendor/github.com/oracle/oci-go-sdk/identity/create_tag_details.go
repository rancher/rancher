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

// CreateTagDetails The representation of CreateTagDetails
type CreateTagDetails struct {

	// The name you assign to the tag during creation. This is the tag key definition.
	// The name must be unique within the tag namespace and cannot be changed.
	Name *string `mandatory:"true" json:"name"`

	// The description you assign to the tag during creation.
	Description *string `mandatory:"true" json:"description"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no predefined name, type, or namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// Defined tags for this resource. Each key is predefined and scoped to a namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// Indicates whether the tag is enabled for cost tracking.
	IsCostTracking *bool `mandatory:"false" json:"isCostTracking"`

	// The tag must have a value type, which is specified with a validator. Tags can use either a
	// static value or a list of possible values. Static values are entered by a user applying the tag
	// to a resource. Lists are created by you and the user must apply a value from the list. Lists
	// are validiated.
	// If you use the default validiator (or don't define a validator), the user applying the tag
	// enters a value. No additional validation is performed.
	// To clear the validator, call UpdateTag with
	// DefaultTagDefinitionValidator (https://docs.cloud.oracle.com/api/#/en/identity/latest/datatypes/DefaultTagDefinitionValidator).
	Validator BaseTagDefinitionValidator `mandatory:"false" json:"validator"`
}

func (m CreateTagDetails) String() string {
	return common.PointerString(m)
}

// UnmarshalJSON unmarshals from json
func (m *CreateTagDetails) UnmarshalJSON(data []byte) (e error) {
	model := struct {
		FreeformTags   map[string]string                 `json:"freeformTags"`
		DefinedTags    map[string]map[string]interface{} `json:"definedTags"`
		IsCostTracking *bool                             `json:"isCostTracking"`
		Validator      basetagdefinitionvalidator        `json:"validator"`
		Name           *string                           `json:"name"`
		Description    *string                           `json:"description"`
	}{}

	e = json.Unmarshal(data, &model)
	if e != nil {
		return
	}
	var nn interface{}
	m.FreeformTags = model.FreeformTags

	m.DefinedTags = model.DefinedTags

	m.IsCostTracking = model.IsCostTracking

	nn, e = model.Validator.UnmarshalPolymorphicJSON(model.Validator.JsonData)
	if e != nil {
		return
	}
	if nn != nil {
		m.Validator = nn.(BaseTagDefinitionValidator)
	} else {
		m.Validator = nil
	}

	m.Name = model.Name

	m.Description = model.Description
	return
}
