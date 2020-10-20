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

// DefaultTagDefinitionValidator Use this validator to clear any existing validator on the tag key definition with the UpdateTag
// operation. Using this `validatorType` is the same as not setting any value on the validator field.
// The resultant value for `validatorType` returned in the response body is `null`.
type DefaultTagDefinitionValidator struct {
}

func (m DefaultTagDefinitionValidator) String() string {
	return common.PointerString(m)
}

// MarshalJSON marshals to json representation
func (m DefaultTagDefinitionValidator) MarshalJSON() (buff []byte, e error) {
	type MarshalTypeDefaultTagDefinitionValidator DefaultTagDefinitionValidator
	s := struct {
		DiscriminatorParam string `json:"validatorType"`
		MarshalTypeDefaultTagDefinitionValidator
	}{
		"DEFAULT",
		(MarshalTypeDefaultTagDefinitionValidator)(m),
	}

	return json.Marshal(&s)
}
