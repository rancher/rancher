package client

const (
	InjectAbortType            = "injectAbort"
	InjectAbortFieldHTTPStatus = "httpStatus"
	InjectAbortFieldPercent    = "percent"
)

type InjectAbort struct {
	HTTPStatus int64 `json:"httpStatus,omitempty" yaml:"httpStatus,omitempty"`
	Percent    int64 `json:"percent,omitempty" yaml:"percent,omitempty"`
}
