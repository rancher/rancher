package client

const (
	ETCDServiceType           = "etcdService"
	ETCDServiceFieldExtraArgs = "extraArgs"
	ETCDServiceFieldImage     = "image"
)

type ETCDService struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	Image     string            `json:"image,omitempty"`
}
