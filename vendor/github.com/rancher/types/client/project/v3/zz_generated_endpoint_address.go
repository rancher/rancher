package client

const (
	EndpointAddressType           = "endpointAddress"
	EndpointAddressFieldHostname  = "hostname"
	EndpointAddressFieldIP        = "ip"
	EndpointAddressFieldNodeName  = "nodeName"
	EndpointAddressFieldTargetRef = "targetRef"
)

type EndpointAddress struct {
	Hostname  string           `json:"hostname,omitempty"`
	IP        string           `json:"ip,omitempty"`
	NodeName  string           `json:"nodeName,omitempty"`
	TargetRef *ObjectReference `json:"targetRef,omitempty"`
}
