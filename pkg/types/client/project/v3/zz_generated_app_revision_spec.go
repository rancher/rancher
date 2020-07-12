package client

const (
	AppRevisionSpecType           = "appRevisionSpec"
	AppRevisionSpecFieldProjectID = "projectId"
)

type AppRevisionSpec struct {
	ProjectID string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
}
