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

// BaseTagDefinitionValidator Validates a definedTag value. Each validator performs validation steps in addition to the standard
// validation for definedTag values. For more information, see
// Limits on Tags (https://docs.cloud.oracle.com/Content/Identity/Concepts/taggingoverview.htm#Limits).
// If you define a validator after a value has been set for a defined tag, then any updates that
// attempt to change the value must pass the additional validation defined by the current rule.
// Previously set values (even those that would fail the current validation) are not updated. You can
// still update other attributes to resources that contain a non-valid defined tag.
// To clear the validator call UpdateTag with
// DefaultTagDefinitionValidator (https://docs.cloud.oracle.com/api/#/en/identity/latest/datatypes/DefaultTagDefinitionValidator).
type BaseTagDefinitionValidator interface {
}

type basetagdefinitionvalidator struct {
	JsonData      []byte
	ValidatorType string `json:"validatorType"`
}

// UnmarshalJSON unmarshals json
func (m *basetagdefinitionvalidator) UnmarshalJSON(data []byte) error {
	m.JsonData = data
	type Unmarshalerbasetagdefinitionvalidator basetagdefinitionvalidator
	s := struct {
		Model Unmarshalerbasetagdefinitionvalidator
	}{}
	err := json.Unmarshal(data, &s.Model)
	if err != nil {
		return err
	}
	m.ValidatorType = s.Model.ValidatorType

	return err
}

// UnmarshalPolymorphicJSON unmarshals polymorphic json
func (m *basetagdefinitionvalidator) UnmarshalPolymorphicJSON(data []byte) (interface{}, error) {

	if data == nil || string(data) == "null" {
		return nil, nil
	}

	var err error
	switch m.ValidatorType {
	case "DEFAULT":
		mm := DefaultTagDefinitionValidator{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	case "ENUM":
		mm := EnumTagDefinitionValidator{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	default:
		return *m, nil
	}
}

func (m basetagdefinitionvalidator) String() string {
	return common.PointerString(m)
}

// BaseTagDefinitionValidatorValidatorTypeEnum Enum with underlying type: string
type BaseTagDefinitionValidatorValidatorTypeEnum string

// Set of constants representing the allowable values for BaseTagDefinitionValidatorValidatorTypeEnum
const (
	BaseTagDefinitionValidatorValidatorTypeEnumvalue BaseTagDefinitionValidatorValidatorTypeEnum = "ENUM"
	BaseTagDefinitionValidatorValidatorTypeDefault   BaseTagDefinitionValidatorValidatorTypeEnum = "DEFAULT"
)

var mappingBaseTagDefinitionValidatorValidatorType = map[string]BaseTagDefinitionValidatorValidatorTypeEnum{
	"ENUM":    BaseTagDefinitionValidatorValidatorTypeEnumvalue,
	"DEFAULT": BaseTagDefinitionValidatorValidatorTypeDefault,
}

// GetBaseTagDefinitionValidatorValidatorTypeEnumValues Enumerates the set of values for BaseTagDefinitionValidatorValidatorTypeEnum
func GetBaseTagDefinitionValidatorValidatorTypeEnumValues() []BaseTagDefinitionValidatorValidatorTypeEnum {
	values := make([]BaseTagDefinitionValidatorValidatorTypeEnum, 0)
	for _, v := range mappingBaseTagDefinitionValidatorValidatorType {
		values = append(values, v)
	}
	return values
}
