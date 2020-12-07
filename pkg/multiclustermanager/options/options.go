package options

type Options struct {
	LocalClusterEnabled bool
	RemoveLocalCluster  bool
	Embedded            bool
	HTTPSListenPort     int
	Debug               bool
	Trace               bool
}
