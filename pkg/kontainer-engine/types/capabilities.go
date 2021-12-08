package types

const (
	GetVersionCapability     = iota
	SetVersionCapability     = iota
	GetClusterSizeCapability = iota
	SetClusterSizeCapability = iota
	EtcdBackupCapability     = iota
)

func (c *Capabilities) AddCapability(cap int64) {
	c.Capabilities[cap] = true
}

func (c *Capabilities) HasGetVersionCapability() bool {
	return c.Capabilities[GetVersionCapability]
}

func (c *Capabilities) HasSetVersionCapability() bool {
	return c.Capabilities[SetVersionCapability]
}

func (c *Capabilities) HasGetClusterSizeCapability() bool {
	return c.Capabilities[GetClusterSizeCapability]
}

func (c *Capabilities) HasSetClusterSizeCapability() bool {
	return c.Capabilities[SetClusterSizeCapability]
}

func (c *Capabilities) HasEtcdBackupCapability() bool {
	return c.Capabilities[EtcdBackupCapability]
}
