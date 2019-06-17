/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Gateway describes a load balancer operating at the edge of the mesh
// receiving incoming or outgoing HTTP/TCP connections. The specification
// describes a set of ports that should be exposed, the type of protocol to
// use, SNI configuration for the load balancer, etc.
//
// For example, the following gateway spec sets up a proxy to act as a load
// balancer exposing port 80 and 9080 (http), 443 (https), and port 2379
// (TCP) for ingress.  The gateway will be applied to the proxy running on
// a pod with labels "app: my-gateway-controller". While Istio will configure the
// proxy to listen on these ports, it is the responsibility of the user to
// ensure that external traffic to these ports are allowed into the mesh.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: Gateway
//     metadata:
//       name: my-gateway
//     spec:
//       selector:
//         app: my-gatweway-controller
//       servers:
//       - port:
//           number: 80
//           name: http
//           protocol: HTTP
//         hosts:
//         - uk.bookinfo.com
//         - eu.bookinfo.com
//         tls:
//           httpsRedirect: true # sends 302 redirect for http requests
//       - port:
//           number: 443
//           name: https
//           protocol: HTTPS
//         hosts:
//         - uk.bookinfo.com
//         - eu.bookinfo.com
//         tls:
//           mode: SIMPLE #enables HTTPS on this port
//           serverCertificate: /etc/certs/servercert.pem
//           privateKey: /etc/certs/privatekey.pem
//       - port:
//           number: 9080
//           name: http-wildcard
//           protocol: HTTP
//         # no hosts implies wildcard match
//       - port:
//           number: 2379 #to expose internal service via external port 2379
//           name: mongo
//           protocol: MONGO
//
// The gateway specification above describes the L4-L6 properties of a load
// balancer. A VirtualService can then be bound to a gateway to control
// the forwarding of traffic arriving at a particular host or gateway port.
//
// For example, the following VirtualService splits traffic for
// https://uk.bookinfo.com/reviews, https://eu.bookinfo.com/reviews,
// http://uk.bookinfo.com:9080/reviews, http://eu.bookinfo.com:9080/reviews
// into two versions (prod and qa) of an internal reviews service on port
// 9080. In addition, requests containing the cookie user: dev-123 will be
// sent to special port 7777 in the qa version. The same rule is also
// applicable inside the mesh for requests to the reviews.prod
// service. This rule is applicable across ports 443, 9080. Note that
// http://uk.bookinfo.com gets redirected to https://uk.bookinfo.com
// (i.e. 80 redirects to 443).
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: bookinfo-rule
//     spec:
//       hosts:
//       - reviews.prod
//       - uk.bookinfo.com
//       - eu.bookinfo.com
//       gateways:
//       - my-gateway
//       - mesh # applies to all the sidecars in the mesh
//       http:
//       - match:
//         - headers:
//             cookie:
//               user: dev-123
//         route:
//         - destination:
//             port:
//               number: 7777
//             name: reviews.qa
//       - match:
//           uri:
//             prefix: /reviews/
//         route:
//         - destination:
//             port:
//               number: 9080 # can be omitted if its the only port for reviews
//             name: reviews.prod
//           weight: 80
//         - destination:
//             name: reviews.qa
//           weight: 20
//
// The following VirtualService forwards traffic arriving at (external) port
// 2379 from 172.17.16.0/24 subnet to internal Mongo server on port 5555. This
// rule is not applicable internally in the mesh as the gateway list omits
// the reserved name "mesh".
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: bookinfo-Mongo
//     spec:
//       hosts:
//       - mongosvr #name of Mongo service
//       gateways:
//       - my-gateway
//       tcp:
//       - match:
//         - port:
//             number: 2379
//           sourceSubnet: "172.17.16.0/24"
//         route:
//         - destination:
//             name: mongo.prod
//
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GatewaySpec `json:"spec"`
}

type GatewaySpec struct {
	// REQUIRED: A list of server specifications.
	Servers []Server `json:"servers"`

	// One or more labels that indicate a specific set of pods/VMs
	// on which this gateway configuration should be applied.
	// If no selectors are provided, the gateway will be implemented by
	// the default istio-ingress controller.
	Selector map[string]string `json:"selector,omitempty"`
}

// Server describes the properties of the proxy on a given load balancer port.
// For example,
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: Gateway
//     metadata:
//       name: my-ingress
//     spec:
//       selector:
//         app: my-ingress-controller
//       servers:
//       - port:
//           number: 80
//           name: http2
//           protocol: HTTP2
//
// Another example
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: Gateway
//     metadata:
//       name: my-tcp-ingress
//     spec:
//       selector:
//         app: my-tcp-ingress-controller
//       servers:
//       - port:
//           number: 27018
//           name: mongo
//           protocol: MONGO
//
// The following is an example of TLS configuration for port 443
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: Gateway
//     metadata:
//       name: my-tls-ingress
//     spec:
//       selector:
//         app: my-tls-ingress-controller
//       servers:
//       - port:
//           number: 443
//           name: https
//           protocol: HTTPS
//         tls:
//           mode: SIMPLE
//           serverCertificate: /etc/certs/server.pem
//           privateKey: /etc/certs/privatekey.pem
//
type Server struct {
	// REQUIRED: The Port on which the proxy should listen for incoming
	// connections
	Port Port `json:"port"`

	// A list of hosts exposed by this gateway. While
	// typically applicable to HTTP services, it can also be used for TCP
	// services using TLS with SNI. Standard DNS wildcard prefix syntax
	// is permitted.
	//
	// A VirtualService that is bound to a gateway must having a matching host
	// in its default destination. Specifically one of the VirtualService
	// destination hosts is a strict suffix of a gateway host or
	// a gateway host is a suffix of one of the VirtualService hosts.
	Hosts []string `json:"hosts,omitempty"`

	// Set of TLS related options that govern the server's behavior. Use
	// these options to control if all http requests should be redirected to
	// https, and the TLS modes to use.
	TLS *TLSOptions `json:"tls,omitempty"`
}

type TLSOptions struct {
	// If set to true, the load balancer will send a 302 redirect for all
	// http connections, asking the clients to use HTTPS.
	HTTPSRedirect bool `json:"httpsRedirect"`

	// Optional: Indicates whether connections to this port should be
	// secured using TLS. The value of this field determines how TLS is
	// enforced.
	Mode TLSMode `json:"mode,omitempty"`

	// REQUIRED if mode is "SIMPLE" or "MUTUAL". The path to the file
	// holding the server-side TLS certificate to use.
	ServerCertificate string `json:"serverCertificate"`

	// REQUIRED if mode is "SIMPLE" or "MUTUAL". The path to the file
	// holding the server's private key.
	PrivateKey string `json:"privateKey"`

	// REQUIRED if mode is "MUTUAL". The path to a file containing
	// certificate authority certificates to use in verifying a presented
	// client side certificate.
	CaCertificates string `json:"caCertificates"`

	// The credentialName stands for a unique identifier that can be used
	// to identify the serverCertificate and the privateKey. The
	// credentialName appended with suffix "-cacert" is used to identify
	// the CaCertificates associated with this server. Gateway workloads
	// capable of fetching credentials from a remote credential store such
	// as Kubernetes secrets, will be configured to retrieve the
	// serverCertificate and the privateKey using credentialName, instead
	// of using the file system paths specified above. If using mutual TLS,
	// gateway workload instances will retrieve the CaCertificates using
	// credentialName-cacert. The semantics of the name are platform
	// dependent.  In Kubernetes, the default Istio supplied credential
	// server expects the credentialName to match the name of the
	// Kubernetes secret that holds the server certificate, the private
	// key, and the CA certificate (if using mutual TLS). Set the
	// `ISTIO_META_USER_SDS` metadata variable in the gateway's proxy to
	// enable the dynamic credential fetching feature.
	CredentialName string `json:"credentialName,omitempty"`

	// A list of alternate names to verify the subject identity in the
	// certificate presented by the client.
	SubjectAltNames []string `json:"subjectAltNames"`
}

// TLS modes enforced by the proxy
type TLSMode string

const (
	// If set to "PASSTHROUGH", the proxy will forward the connection
	// to the upstream server selected based on the SNI string presented
	// by the client.
	TLSModePassThrough TLSMode = "PASSTHROUGH"

	// If set to "SIMPLE", the proxy will secure connections with
	// standard TLS semantics.
	TLSModeSimple TLSMode = "SIMPLE"

	// If set to "MUTUAL", the proxy will secure connections to the
	// upstream using mutual TLS by presenting client certificates for
	// authentication.
	TLSModeMutual TLSMode = "MUTUAL"
)

// Port describes the properties of a specific port of a service.
type Port struct {
	// REQUIRED: A valid non-negative integer port number.
	Number int `json:"number"`

	// REQUIRED: The protocol exposed on the port.
	// MUST BE one of HTTP|HTTPS|GRPC|HTTP2|MONGO|TCP.
	Protocol PortProtocol `json:"protocol"`

	// Label assigned to the port.
	Name string `json:"name,omitempty"`
}

type PortProtocol string

const (
	ProtocolHTTP  PortProtocol = "HTTP"
	ProtocolHTTPS PortProtocol = "HTTPS"
	ProtocolGRPC  PortProtocol = "GRPC"
	ProtocolHTTP2 PortProtocol = "HTTP2"
	ProtocolMongo PortProtocol = "Mongo"
	ProtocolTCP   PortProtocol = "TCP"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GatewayList is a list of Gateway resources
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Gateway `json:"items"`
}
