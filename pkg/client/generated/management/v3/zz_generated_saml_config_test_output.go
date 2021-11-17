package client


	


import (
	
)

const (
    SamlConfigTestOutputType = "samlConfigTestOutput"
	SamlConfigTestOutputFieldIdpRedirectURL = "idpRedirectUrl"
)

type SamlConfigTestOutput struct {
        IdpRedirectURL string `json:"idpRedirectUrl,omitempty" yaml:"idpRedirectUrl,omitempty"`
}

