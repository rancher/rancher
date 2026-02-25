package v3

type Member struct {
	UserName           string `json:"userName,omitempty" norman:"type=reference[user]"`
	UserPrincipalName  string `json:"userPrincipalName,omitempty" norman:"type=reference[principal]"`
	DisplayName        string `json:"displayName,omitempty"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"type=reference[principal]"`
	AccessType         string `json:"accessType,omitempty" norman:"type=enum,options=owner|member|read-only"`
}
