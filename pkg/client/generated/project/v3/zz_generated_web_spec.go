package client

const (
	WebSpecType           = "webSpec"
	WebSpecFieldPageTitle = "pageTitle"
)

type WebSpec struct {
	PageTitle string `json:"pageTitle,omitempty" yaml:"pageTitle,omitempty"`
}
