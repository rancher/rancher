package googleoauth

// Account defines properties an account on github has
type Account struct {
	Email           string `json:"email,omitempty"`
	Name            string `json:"name,omitempty"`
	GivenName       string `json:"given_name,omitempty"`
	FamilyName      string `json:"family_name,omitempty"`
	PictureURL      string `json:"picture,omitempty"`
	SubjectUniqueID string `json:"sub,omitempty"`
	EmailVerified   bool   `json:"email_verified,omitempty"`
	HostedDomain    string `json:"hd,omitempty"`
	Type            string
}
