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
	Causes            []StatusCause `json:"causes,omitempty"`
	Group             string        `json:"group,omitempty"`
	Kind              string        `json:"kind,omitempty"`
	Name              string        `json:"name,omitempty"`
	RetryAfterSeconds *int64        `json:"retryAfterSeconds,omitempty"`
	UID               string        `json:"uid,omitempty"`
}
