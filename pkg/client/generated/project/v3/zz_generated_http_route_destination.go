package client

const (
	HTTPRouteDestinationType             = "httpRouteDestination"
	HTTPRouteDestinationFieldDestination = "destination"
	HTTPRouteDestinationFieldHeaders     = "headers"
	HTTPRouteDestinationFieldWeight      = "weight"
)

type HTTPRouteDestination struct {
	Destination *Destination `json:"destination,omitempty" yaml:"destination,omitempty"`
	Headers     *Headers     `json:"headers,omitempty" yaml:"headers,omitempty"`
	Weight      int64        `json:"weight,omitempty" yaml:"weight,omitempty"`
}
