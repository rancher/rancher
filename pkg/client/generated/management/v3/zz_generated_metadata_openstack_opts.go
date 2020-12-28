package client

const (
	MetadataOpenstackOptsType                = "metadataOpenstackOpts"
	MetadataOpenstackOptsFieldRequestTimeout = "request-timeout"
	MetadataOpenstackOptsFieldSearchOrder    = "search-order"
)

type MetadataOpenstackOpts struct {
	RequestTimeout int64  `json:"request-timeout,omitempty" yaml:"request-timeout,omitempty"`
	SearchOrder    string `json:"search-order,omitempty" yaml:"search-order,omitempty"`
}
