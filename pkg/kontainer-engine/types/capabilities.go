package types

const (
	GetVersionCapability     = iota
	SetVersionCapability     = iota
	GetClusterSizeCapability = iota
	EtcdBackupCapability     = iota
)

func (c *Capabilities) AddCapability(cap int64) {
	c.Capabilities[cap] = true
}
