package types

import "github.com/rancher/types/apis/management.cattle.io/v3"

type Config struct {
	ACMEDomains       []string
	AddLocal          string
	Embedded          bool
	KubeConfig        string
	HTTPListenPort    int
	HTTPSListenPort   int
	K8sMode           string
	Debug             bool
	NoCACerts         bool
	ListenConfig      *v3.ListenConfig `json:"-"`
	AuditLogPath      string
	AuditLogMaxage    int
	AuditLogMaxsize   int
	AuditLogMaxbackup int
	AuditLevel        int
}
