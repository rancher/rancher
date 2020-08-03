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

// EnumTagDefinitionValidator Used to validate the value set for a defined tag and contains the list of allowable `values`.
// You must specify at least one valid value in the `values` array. You can't have blank or
// or empty strings (`""`). Duplicate values are not allowed.
type EnumTagDefinitionValidator struct {

	// The list of allowed values for a definedTag value.
	Values []string `mandatory:"false" json:"values"`
}

func (m EnumTagDefinitionValidator) String() string {
	return common.PointerString(m)
}

// MarshalJSON marshals to json representation
func (m EnumTagDefinitionValidator) MarshalJSON() (buff []byte, e error) {
	type MarshalTypeEnumTagDefinitionValidator EnumTagDefinitionValidator
	s := struct {
		DiscriminatorParam string `json:"validatorType"`
		MarshalTypeEnumTagDefinitionValidator
	}{
		"ENUM",
		(MarshalTypeEnumTagDefinitionValidator)(m),
	}

	return json.Marshal(&s)
}
