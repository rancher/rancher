package azuread

type searchResult struct {
	Items []Account `json:"value"`
}

type Account struct {
	ObjectID          string `json:"objectId,omitempty"`
	AccountName       string `json:"accountName,omitempty"`
	UserPrincipalName string `json:"userPrincipalName,omitempty"`
	ThumbNail         string `json:"thumbNail,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
}
