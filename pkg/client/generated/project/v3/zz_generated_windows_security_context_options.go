package client

const (
	WindowsSecurityContextOptionsType                        = "windowsSecurityContextOptions"
	WindowsSecurityContextOptionsFieldGMSACredentialSpec     = "gmsaCredentialSpec"
	WindowsSecurityContextOptionsFieldGMSACredentialSpecName = "gmsaCredentialSpecName"
	WindowsSecurityContextOptionsFieldHostProcess            = "hostProcess"
	WindowsSecurityContextOptionsFieldRunAsUserName          = "runAsUserName"
)

type WindowsSecurityContextOptions struct {
	GMSACredentialSpec     string `json:"gmsaCredentialSpec,omitempty" yaml:"gmsaCredentialSpec,omitempty"`
	GMSACredentialSpecName string `json:"gmsaCredentialSpecName,omitempty" yaml:"gmsaCredentialSpecName,omitempty"`
	HostProcess            *bool  `json:"hostProcess,omitempty" yaml:"hostProcess,omitempty"`
	RunAsUserName          string `json:"runAsUserName,omitempty" yaml:"runAsUserName,omitempty"`
}
