package client

const (
	SearchPrincipalsInputType      = "searchPrincipalsInput"
	SearchPrincipalsInputFieldName = "name"
)

type SearchPrincipalsInput struct {
	Name string `json:"name,omitempty"`
}
