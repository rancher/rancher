package client

const (
	CisScanStatusType               = "cisScanStatus"
	CisScanStatusFieldFail          = "fail"
	CisScanStatusFieldNotApplicable = "notApplicable"
	CisScanStatusFieldPass          = "pass"
	CisScanStatusFieldSkip          = "skip"
	CisScanStatusFieldTotal         = "total"
)

type CisScanStatus struct {
	Fail          int64 `json:"fail,omitempty" yaml:"fail,omitempty"`
	NotApplicable int64 `json:"notApplicable,omitempty" yaml:"notApplicable,omitempty"`
	Pass          int64 `json:"pass,omitempty" yaml:"pass,omitempty"`
	Skip          int64 `json:"skip,omitempty" yaml:"skip,omitempty"`
	Total         int64 `json:"total,omitempty" yaml:"total,omitempty"`
}
