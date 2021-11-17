package client


	


import (
	
)

const (
    OIDCTestOutputType = "oidcTestOutput"
	OIDCTestOutputFieldRedirectURL = "redirectUrl"
)

type OIDCTestOutput struct {
        RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}

