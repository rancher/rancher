package client

const (
	KeyToPathType      = "keyToPath"
	KeyToPathFieldKey  = "key"
	KeyToPathFieldMode = "mode"
	KeyToPathFieldPath = "path"
)

type KeyToPath struct {
	Key  string `json:"key,omitempty"`
	Mode *int64 `json:"mode,omitempty"`
	Path string `json:"path,omitempty"`
}
