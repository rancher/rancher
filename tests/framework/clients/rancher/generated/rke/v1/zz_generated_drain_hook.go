package client

const (
	DrainHookType            = "drainHook"
	DrainHookFieldAnnotation = "annotation"
)

type DrainHook struct {
	Annotation string `json:"annotation,omitempty" yaml:"annotation,omitempty"`
}
