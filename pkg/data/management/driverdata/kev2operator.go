package driverdata

import (
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

const (
	EKSOperator     = "eks"
	GKEOperator     = "gke"
	AKSOperator     = "aks"
	AlibabaOperator = "alibaba"
)

type CredentialFields map[string]v32.Field

// Credential Fields data for KEv2 Operators which don't have a corresponding node driver.
var KEv2OperatorsCredentialFields = map[string]CredentialFields{
	AlibabaOperator: {
		"accessKeyId": v32.Field{
			Create: true,
			Update: true,
			Type:   "string",
		},
		"accessKeySecret": v32.Field{
			Create: true,
			Update: true,
			Type:   "password",
		},
	},
}
