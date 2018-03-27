package client

const (
	StatusDetailsType                   = "statusDetails"
	StatusDetailsFieldCauses            = "causes"
	StatusDetailsFieldGroup             = "group"
	StatusDetailsFieldKind              = "kind"
	StatusDetailsFieldName              = "name"
	StatusDetailsFieldRetryAfterSeconds = "retryAfterSeconds"
	StatusDetailsFieldUID               = "uid"
)

type StatusDetails struct {
	Causes            []StatusCause `json:"causes,omitempty" yaml:"causes,omitempty"`
	Group             string        `json:"group,omitempty" yaml:"group,omitempty"`
	Kind              string        `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name              string        `json:"name,omitempty" yaml:"name,omitempty"`
	RetryAfterSeconds int64         `json:"retryAfterSeconds,omitempty" yaml:"retryAfterSeconds,omitempty"`
	UID               string        `json:"uid,omitempty" yaml:"uid,omitempty"`
}
