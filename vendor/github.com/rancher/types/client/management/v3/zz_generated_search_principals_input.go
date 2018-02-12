package client

const (
	SearchPrincipalsInputType               = "searchPrincipalsInput"
	SearchPrincipalsInputFieldName          = "name"
	SearchPrincipalsInputFieldPrincipalType = "principalType"
)

type SearchPrincipalsInput struct {
	Name          string `json:"name,omitempty"`
	PrincipalType string `json:"principalType,omitempty"`
}
