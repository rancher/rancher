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
	"github.com/knative/pkg/apis/istio/common/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualService
type VirtualService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VirtualServiceSpec `json:"spec"`
}

// A VirtualService defines a set of traffic routing rules to apply when a host is
// addressed. Each routing rule defines matching criteria for traffic of a specific
// protocol. If the traffic is matched, then it is sent to a named destination service
// (or subset/version of it) defined in the registry.
//
// The source of traffic can also be matched in a routing rule. This allows routing
// to be customized for specific client contexts.
//
// The following example routes all HTTP traffic by default to
// pods of the reviews service with label "version: v1". In addition,
// HTTP requests containing /wpcatalog/, /consumercatalog/ url prefixes will
// be rewritten to /newcatalog and sent to pods with label "version: v2". The
// rules will be applied at the gateway named "bookinfo" as well as at all
// the sidecars in the mesh (indicated by the reserved gateway name
// "mesh").
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: reviews-route
//     spec:
//       hosts:
//       - reviews
//       gateways: # if omitted, defaults to "mesh"
//       - bookinfo
//       - mesh
//       http:
//       - match:
//         - uri:
//             prefix: "/wpcatalog"
//         - uri:
//             prefix: "/consumercatalog"
//         rewrite:
//           uri: "/newcatalog"
//         route:
//         - destination:
//             host: reviews
//             subset: v2
//       - route:
//         - destination:
//             host: reviews
//             subset: v1
//
// A subset/version of a route destination is identified with a reference
// to a named service subset which must be declared in a corresponding
// DestinationRule.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: DestinationRule
//     metadata:
//       name: reviews-destination
//     spec:
//       host: reviews
//       subsets:
//       - name: v1
//         labels:
//           version: v1
//       - name: v2
//         labels:
//           version: v2
//
// A host name can be defined by only one VirtualService. A single
// VirtualService can be used to describe traffic properties for multiple
// HTTP and TCP ports.
type VirtualServiceSpec struct {
	// REQUIRED. The destination address for traffic captured by this virtual
	// service. Could be a DNS name with wildcard prefix or a CIDR
	// prefix. Depending on the platform, short-names can also be used
	// instead of a FQDN (i.e. has no dots in the name). In such a scenario,
	// the FQDN of the host would be derived based on the underlying
	// platform.
	//
	// For example on Kubernetes, when hosts contains a short name, Istio will
	// interpret the short name based on the namespace of the rule. Thus, when a
	// client namespace applies a rule in the "default" namespace containing a name
	// "reviews, Istio will setup routes to the "reviews.default.svc.cluster.local"
	// service. However, if a different name such as "reviews.sales.svc.cluster.local"
	// is used, it would be treated as a FQDN during virtual host matching.
	// In Consul, a plain service name would be resolved to the FQDN
	// "reviews.service.consul".
	//
	// Note that the hosts field applies to both HTTP and TCP
	// services. Service inside the mesh, i.e., those found in the service
	// registry, must always be referred to using their alphanumeric
	// names. IP addresses or CIDR prefixes are allowed only for services
	// defined via the Gateway.
	Hosts []string `json:"hosts"`

	// The names of gateways and sidecars that should apply these routes. A
	// single VirtualService is used for sidecars inside the mesh as well
	// as for one or more gateways. The selection condition imposed by this field
	// can be overridden using the source field in the match conditions of HTTP/TCP
	// routes. The reserved word "mesh" is used to imply all the sidecars in
	// the mesh. When this field is omitted, the default gateway ("mesh")
	// will be used, which would apply the rule to all sidecars in the
	// mesh. If a list of gateway names is provided, the rules will apply
	// only to the gateways. To apply the rules to both gateways and sidecars,
	// specify "mesh" as one of the gateway names.
	Gateways []string `json:"gateways,omitempty"`

	// An ordered list of route rules for HTTP traffic.
	// The first rule matching an incoming request is used.
	HTTP []HTTPRoute `json:"http,omitempty"`

	// An ordered list of route rules for TCP traffic.
	// The first rule matching an incoming request is used.
	TCP []TCPRoute `json:"tcp,omitempty"`

	TLS []TLSRoute `json:"tls,omitempty"`
}

// Describes match conditions and actions for routing HTTP/1.1, HTTP2, and
// gRPC traffic. See VirtualService for usage examples.
type HTTPRoute struct {
	// Match conditions to be satisfied for the rule to be
	// activated. All conditions inside a single match block have AND
	// semantics, while the list of match blocks have OR semantics. The rule
	// is matched if any one of the match blocks succeed.
	Match []HTTPMatchRequest `json:"match,omitempty"`

	// A http rule can either redirect or forward (default) traffic. The
	// forwarding target can be one of several versions of a service (see
	// glossary in beginning of document). Weights associated with the
	// service version determine the proportion of traffic it receives.
	Route []HTTPRouteDestination `json:"route,omitempty"`

	// A http rule can either redirect or forward (default) traffic. If
	// traffic passthrough option is specified in the rule,
	// route/redirect will be ignored. The redirect primitive can be used to
	// send a HTTP 302 redirect to a different URI or Authority.
	Redirect *HTTPRedirect `json:"redirect,omitempty"`

	// Rewrite HTTP URIs and Authority headers. Rewrite cannot be used with
	// Redirect primitive. Rewrite will be performed before forwarding.
	Rewrite *HTTPRewrite `json:"rewrite,omitempty"`

	// Indicates that a HTTP/1.1 client connection to this particular route
	// should be allowed (and expected) to upgrade to a WebSocket connection.
	// The default is false. Istio's reference sidecar implementation (Envoy)
	// expects the first request to this route to contain the WebSocket
	// upgrade headers. Otherwise, the request will be rejected. Note that
	// Websocket allows secondary protocol negotiation which may then be
	// subject to further routing rules based on the protocol selected.
	WebsocketUpgrade bool `json:"websocketUpgrade,omitempty"`

	// Timeout for HTTP requests.
	Timeout string `json:"timeout,omitempty"`

	// Retry policy for HTTP requests.
	Retries *HTTPRetry `json:"retries,omitempty"`

	// Fault injection policy to apply on HTTP traffic.
	Fault *HTTPFaultInjection `json:"fault,omitempty"`

	// Mirror HTTP traffic to a another destination in addition to forwarding
	// the requests to the intended destination. Mirrored traffic is on a
	// best effort basis where the sidecar/gateway will not wait for the
	// mirrored cluster to respond before returning the response from the
	// original destination.  Statistics will be generated for the mirrored
	// destination.
	Mirror *Destination `json:"mirror,omitempty"`

	// Additional HTTP headers to add before forwarding a request to the
	// destination service.
	DeprecatedAppendHeaders map[string]string `json:"appendHeaders,omitempty"`

	// Header manipulation rules
	Headers *Headers `json:"headers,omitempty"`

	// Http headers to remove before returning the response to the caller
	RemoveResponseHeaders map[string]string `json:"removeResponseHeaders,omitempty"`

	// Cross-Origin Resource Sharing policy
	CorsPolicy *CorsPolicy `json:"corsPolicy,omitempty"`
}

// Headers describes header manipulation rules.
type Headers struct {
	// Header manipulation rules to apply before forwarding a request
	// to the destination service
	Request *HeaderOperations `json:"request,omitempty"`

	// Header manipulation rules to apply before returning a response
	// to the caller
	Response *HeaderOperations `json:"response,omitempty"`
}

// HeaderOperations Describes the header manipulations to apply
type HeaderOperations struct {
	// Overwrite the headers specified by key with the given values
	Set map[string]string `json:"set,omitempty"`

	// Append the given values to the headers specified by keys
	// (will create a comma-separated list of values)
	Add map[string]string `json:"add,omitempty"`

	// Remove a the specified headers
	Remove []string `json:"remove,omitempty"`
}

// HttpMatchRequest specifies a set of criterion to be met in order for the
// rule to be applied to the HTTP request. For example, the following
// restricts the rule to match only requests where the URL path
// starts with /ratings/v2/ and the request contains a "cookie" with value
// "user=jason".
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: ratings-route
//     spec:
//       hosts:
//       - ratings
//       http:
//       - match:
//         - headers:
//             cookie:
//               regex: "^(.*?;)?(user=jason)(;.*)?"
//             uri:
//               prefix: "/ratings/v2/"
//         route:
//         - destination:
//             host: ratings
//
// HTTPMatchRequest CANNOT be empty.
type HTTPMatchRequest struct {
	// URI to match
	// values are case-sensitive and formatted as follows:
	//
	// - `exact: "value"` for exact string match
	//
	// - `prefix: "value"` for prefix-based match
	//
	// - `regex: "value"` for ECMAscript style regex-based match
	//
	URI *v1alpha1.StringMatch `json:"uri,omitempty"`

	// URI Scheme
	// values are case-sensitive and formatted as follows:
	//
	// - `exact: "value"` for exact string match
	//
	// - `prefix: "value"` for prefix-based match
	//
	// - `regex: "value"` for ECMAscript style regex-based match
	//
	Scheme *v1alpha1.StringMatch `json:"scheme,omitempty"`

	// HTTP Method
	// values are case-sensitive and formatted as follows:
	//
	// - `exact: "value"` for exact string match
	//
	// - `prefix: "value"` for prefix-based match
	//
	// - `regex: "value"` for ECMAscript style regex-based match
	//
	Method *v1alpha1.StringMatch `json:"method,omitempty"`

	// HTTP Authority
	// values are case-sensitive and formatted as follows:
	//
	// - `exact: "value"` for exact string match
	//
	// - `prefix: "value"` for prefix-based match
	//
	// - `regex: "value"` for ECMAscript style regex-based match
	//
	Authority *v1alpha1.StringMatch `json:"authority,omitempty"`

	// The header keys must be lowercase and use hyphen as the separator,
	// e.g. _x-request-id_.
	//
	// Header values are case-sensitive and formatted as follows:
	//
	// - `exact: "value"` for exact string match
	//
	// - `prefix: "value"` for prefix-based match
	//
	// - `regex: "value"` for ECMAscript style regex-based match
	//
	// **Note:** The keys `uri`, `scheme`, `method`, and `authority` will be ignored.
	Headers map[string]v1alpha1.StringMatch `json:"headers,omitempty"`

	// Specifies the ports on the host that is being addressed. Many services
	// only expose a single port or label ports with the protocols they support,
	// in these cases it is not required to explicitly select the port.
	Port uint32 `json:"port,omitempty"`

	// One or more labels that constrain the applicability of a rule to
	// workloads with the given labels. If the VirtualService has a list of
	// gateways specified at the top, it should include the reserved gateway
	// `mesh` in order for this field to be applicable.
	SourceLabels map[string]string `json:"sourceLabels,omitempty"`

	// Names of gateways where the rule should be applied to. Gateway names
	// at the top of the VirtualService (if any) are overridden. The gateway match is
	// independent of sourceLabels.
	Gateways []string `json:"gateways,omitempty"`
}

type HTTPRouteDestination struct {
	// REQUIRED. Destination uniquely identifies the instances of a service
	// to which the request/connection should be forwarded to.
	Destination Destination `json:"destination"`

	// REQUIRED. The proportion of traffic to be forwarded to the service
	// version. (0-100). Sum of weights across destinations SHOULD BE == 100.
	// If there is only destination in a rule, the weight value is assumed to
	// be 100.
	Weight int `json:"weight"`

	// Header manipulation rules
	Headers *Headers `json:"headers,omitempty"`
}

// Destination indicates the network addressable service to which the
// request/connection will be sent after processing a routing rule. The
// destination.name should unambiguously refer to a service in the service
// registry. It can be a short name or a fully qualified domain name from
// the service registry, a resolvable DNS name, an IP address or a service
// name from the service registry and a subset name. The order of inference
// is as follows:
//
// 1. Service registry lookup. The entire name is looked up in the service
// registry. If the lookup succeeds, the search terminates. The requests
// will be routed to any instance of the service in the mesh. When the
// service name consists of a single word, the FQDN will be constructed in
// a platform specific manner. For example, in Kubernetes, the namespace
// associated with the routing rule will be used to identify the service as
// <servicename>.<rulenamespace>. However, if the service name contains
// multiple words separated by a dot (e.g., reviews.prod), the name in its
// entirety would be looked up in the service registry.
//
// 2. Runtime DNS lookup by the proxy. If step 1 fails, and the name is not
// an IP address, it will be considered as a DNS name that is not in the
// service registry (e.g., wikipedia.org). The sidecar/gateway will resolve
// the DNS and load balance requests appropriately. See Envoy's strict_dns
// for details.
//
// The following example routes all traffic by default to pods of the
// reviews service with label "version: v1" (i.e., subset v1), and some
// to subset v2, in a kubernetes environment.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: reviews-route
//     spec:
//       hosts:
//       - reviews # namespace is same as the client/caller's namespace
//       http:
//       - match:
//         - uri:
//             prefix: "/wpcatalog"
//         - uri:
//             prefix: "/consumercatalog"
//         rewrite:
//           uri: "/newcatalog"
//         route:
//         - destination:
//             host: reviews
//             subset: v2
//       - route:
//         - destination:
//             host: reviews
//             subset: v1
//
// And the associated DestinationRule
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: DestinationRule
//     metadata:
//       name: reviews-destination
//     spec:
//       host: reviews
//       subsets:
//       - name: v1
//         labels:
//           version: v1
//       - name: v2
//         labels:
//           version: v2
//
// The following VirtualService sets a timeout of 5s for all calls to
// productpage.prod service. Notice that there are no subsets defined in
// this rule. Istio will fetch all instances of productpage.prod service
// from the service registry and populate the sidecar's load balancing
// pool.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: my-productpage-rule
//     spec:
//       hosts:
//       - productpage.prod # in kubernetes, this applies only to prod namespace
//       http:
//       - timeout: 5s
//         route:
//         - destination:
//             host: productpage.prod
//
// The following sets a timeout of 5s for all calls to the external
// service wikipedia.org, as there is no internal service of that name.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: my-wiki-rule
//     spec:
//       hosts:
//       - wikipedia.org
//       http:
//       - timeout: 5s
//         route:
//         - destination:
//             host: wikipedia.org
//
type Destination struct {
	// REQUIRED. The name of a service from the service registry. Service
	// names are looked up from the platform's service registry (e.g.,
	// Kubernetes services, Consul services, etc.) and from the hosts
	// declared by [ServiceEntry](#ServiceEntry). Traffic forwarded to
	// destinations that are not found in either of the two, will be dropped.
	//
	// *Note for Kubernetes users*: When short names are used (e.g. "reviews"
	// instead of "reviews.default.svc.cluster.local"), Istio will interpret
	// the short name based on the namespace of the rule, not the service. A
	// rule in the "default" namespace containing a host "reviews will be
	// interpreted as "reviews.default.svc.cluster.local", irrespective of
	// the actual namespace associated with the reviews service. _To avoid
	// potential misconfigurations, it is recommended to always use fully
	// qualified domain names over short names._
	Host string `json:"host"`

	// The name of a subset within the service. Applicable only to services
	// within the mesh. The subset must be defined in a corresponding
	// DestinationRule.
	Subset string `json:"subset,omitempty"`

	// Specifies the port on the host that is being addressed. If a service
	// exposes only a single port it is not required to explicitly select the
	// port.
	Port PortSelector `json:"port,omitempty"`
}

// PortSelector specifies the number of a port to be used for
// matching or selection for final routing.
type PortSelector struct {
	// Choose one of the fields below.

	// Valid port number
	Number uint32 `json:"number,omitempty"`

	// Valid port name
	Name string `json:"name,omitempty"`
}

// Describes match conditions and actions for routing TCP traffic. The
// following routing rule forwards traffic arriving at port 27017 for
// mongo.prod.svc.cluster.local from 172.17.16.* subnet to another Mongo
// server on port 5555.
//
// ```yaml
// apiVersion: networking.istio.io/v1alpha3
// kind: VirtualService
// metadata:
//   name: bookinfo-Mongo
// spec:
//   hosts:
//   - mongo.prod.svc.cluster.local
//   tcp:
//   - match:
//     - port: 27017
//       sourceSubnet: "172.17.16.0/24"
//     route:
//     - destination:
//         host: mongo.backup.svc.cluster.local
//         port:
//           number: 5555
// ```
type TCPRoute struct {
	// Match conditions to be satisfied for the rule to be
	// activated. All conditions inside a single match block have AND
	// semantics, while the list of match blocks have OR semantics. The rule
	// is matched if any one of the match blocks succeed.
	Match []L4MatchAttributes `json:"match"`

	// The destinations to which the connection should be forwarded to. Weights
	// must add to 100%.
	Route []HTTPRouteDestination `json:"route"`
}

// Describes match conditions and actions for routing unterminated TLS
// traffic (TLS/HTTPS) The following routing rule forwards unterminated TLS
// traffic arriving at port 443 of gateway called mygateway to internal
// services in the mesh based on the SNI value.
//
// ```yaml
// kind: VirtualService
// metadata:
//   name: bookinfo-sni
// spec:
//   hosts:
//   - '*.bookinfo.com'
//   gateways:
//   - mygateway
//   tls:
//   - match:
//     - port: 443
//       sniHosts:
//       - login.bookinfo.com
//     route:
//     - destination:
//         host: login.prod.svc.cluster.local
//   - match:
//     - port: 443
//       sniHosts:
//       - reviews.bookinfo.com
//     route:
//     - destination:
//         host: reviews.prod.svc.cluster.local
// ```
type TLSRoute struct {
	// REQUIRED. Match conditions to be satisfied for the rule to be
	// activated. All conditions inside a single match block have AND
	// semantics, while the list of match blocks have OR semantics. The rule
	// is matched if any one of the match blocks succeed.
	Match []TLSMatchAttributes `json:"match"`

	// The destination to which the connection should be forwarded to.
	Route []HTTPRouteDestination `json:"route"`
}

// L4 connection match attributes. Note that L4 connection matching support
// is incomplete.
type L4MatchAttributes struct {
	// IPv4 or IPv6 ip address of destination with optional subnet.  E.g.,
	// a.b.c.d/xx form or just a.b.c.d.
	DestinationSubnets []string `json:"destinationSubnets,omitempty"`

	// Specifies the port on the host that is being addressed. Many services
	// only expose a single port or label ports with the protocols they support,
	// in these cases it is not required to explicitly select the port.
	Port int `json:"port,omitempty"`

	// One or more labels that constrain the applicability of a rule to
	// workloads with the given labels. If the VirtualService has a list of
	// gateways specified at the top, it should include the reserved gateway
	// `mesh` in order for this field to be applicable.
	SourceLabels map[string]string `json:"sourceLabels,omitempty"`

	// Names of gateways where the rule should be applied to. Gateway names
	// at the top of the VirtualService (if any) are overridden. The gateway match is
	// independent of sourceLabels.
	Gateways []string `json:"gateways,omitempty"`
}

// TLS connection match attributes.
type TLSMatchAttributes struct {
	// REQUIRED. SNI (server name indicator) to match on. Wildcard prefixes
	// can be used in the SNI value, e.g., *.com will match foo.example.com
	// as well as example.com. An SNI value must be a subset (i.e., fall
	// within the domain) of the corresponding virtual service's hosts
	SniHosts []string `json:"sniHosts"`

	// IPv4 or IPv6 ip addresses of destination with optional subnet.  E.g.,
	// a.b.c.d/xx form or just a.b.c.d.
	DestinationSubnets []string `json:"destinationSubnets,omitempty"`

	// Specifies the port on the host that is being addressed. Many services
	// only expose a single port or label ports with the protocols they support,
	// in these cases it is not required to explicitly select the port.
	Port int `json:"port,omitempty"`

	// One or more labels that constrain the applicability of a rule to
	// workloads with the given labels. If the VirtualService has a list of
	// gateways specified at the top, it should include the reserved gateway
	// `mesh` in order for this field to be applicable.
	SourceLabels map[string]string `json:"sourceLabels,omitempty"`

	// Names of gateways where the rule should be applied to. Gateway names
	// at the top of the VirtualService (if any) are overridden. The gateway match is
	// independent of sourceLabels.
	Gateways []string `json:"gateways,omitempty"`
}

// HTTPRedirect can be used to send a 302 redirect response to the caller,
// where the Authority/Host and the URI in the response can be swapped with
// the specified values. For example, the following rule redirects
// requests for /v1/getProductRatings API on the ratings service to
// /v1/bookRatings provided by the bookratings service.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: ratings-route
//     spec:
//       hosts:
//       - ratings
//       http:
//       - match:
//         - uri:
//             exact: /v1/getProductRatings
//       redirect:
//         uri: /v1/bookRatings
//         authority: bookratings.default.svc.cluster.local
//       ...
//
type HTTPRedirect struct {
	// On a redirect, overwrite the Path portion of the URL with this
	// value. Note that the entire path will be replaced, irrespective of the
	// request URI being matched as an exact path or prefix.
	URI string `json:"uri,omitempty"`

	// On a redirect, overwrite the Authority/Host portion of the URL with
	// this value.
	Authority string `json:"authority,omitempty"`
}

// HTTPRewrite can be used to rewrite specific parts of a HTTP request
// before forwarding the request to the destination. Rewrite primitive can
// be used only with the HTTPRouteDestinations. The following example
// demonstrates how to rewrite the URL prefix for api call (/ratings) to
// ratings service before making the actual API call.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: ratings-route
//     spec:
//       hosts:
//       - ratings
//       http:
//       - match:
//         - uri:
//             prefix: /ratings
//         rewrite:
//           uri: /v1/bookRatings
//         route:
//         - destination:
//             host: ratings
//             subset: v1
//
type HTTPRewrite struct {
	// rewrite the path (or the prefix) portion of the URI with this
	// value. If the original URI was matched based on prefix, the value
	// provided in this field will replace the corresponding matched prefix.
	URI string `json:"uri,omitempty"`

	// rewrite the Authority/Host header with this value.
	Authority string `json:"authority,omitempty"`
}

// Describes the retry policy to use when a HTTP request fails. For
// example, the following rule sets the maximum number of retries to 3 when
// calling ratings:v1 service, with a 2s timeout per retry attempt.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: ratings-route
//     spec:
//       hosts:
//       - ratings
//       http:
//       - route:
//         - destination:
//             host: ratings
//             subset: v1
//         retries:
//           attempts: 3
//           perTryTimeout: 2s
//
type HTTPRetry struct {
	// REQUIRED. Number of retries for a given request. The interval
	// between retries will be determined automatically (25ms+). Actual
	// number of retries attempted depends on the httpReqTimeout.
	Attempts int `json:"attempts"`

	// Timeout per retry attempt for a given request. format: 1h/1m/1s/1ms. MUST BE >=1ms.
	PerTryTimeout string `json:"perTryTimeout"`
}

// Describes the Cross-Origin Resource Sharing (CORS) policy, for a given
// service. Refer to
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Access_control_CORS
// for further details about cross origin resource sharing. For example,
// the following rule restricts cross origin requests to those originating
// from example.com domain using HTTP POST/GET, and sets the
// Access-Control-Allow-Credentials header to false. In addition, it only
// exposes X-Foo-bar header and sets an expiry period of 1 day.
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: ratings-route
//     spec:
//       hosts:
//       - ratings
//       http:
//       - route:
//         - destination:
//             host: ratings
//             subset: v1
//         corsPolicy:
//           allowOrigin:
//           - example.com
//           allowMethods:
//           - POST
//           - GET
//           allowCredentials: false
//           allowHeaders:
//           - X-Foo-Bar
//           maxAge: "1d"
//
type CorsPolicy struct {
	// The list of origins that are allowed to perform CORS requests. The
	// content will be serialized into the Access-Control-Allow-Origin
	// header. Wildcard * will allow all origins.
	AllowOrigin []string `json:"allowOrigin,omitempty"`

	// List of HTTP methods allowed to access the resource. The content will
	// be serialized into the Access-Control-Allow-Methods header.
	AllowMethods []string `json:"allowMethods,omitempty"`

	// List of HTTP headers that can be used when requesting the
	// resource. Serialized to Access-Control-Allow-Methods header.
	AllowHeaders []string `json:"allowHeaders,omitempty"`

	// A white list of HTTP headers that the browsers are allowed to
	// access. Serialized into Access-Control-Expose-Headers header.
	ExposeHeaders []string `json:"exposeHeaders,omitempty"`

	// Specifies how long the results of a preflight request can be
	// cached. Translates to the Access-Control-Max-Age header.
	MaxAge string `json:"maxAge,omitempty"`

	// Indicates whether the caller is allowed to send the actual request
	// (not the preflight) using credentials. Translates to
	// Access-Control-Allow-Credentials header.
	AllowCredentials bool `json:"allowCredentials,omitempty"`
}

// HTTPFaultInjection can be used to specify one or more faults to inject
// while forwarding http requests to the destination specified in a route.
// Fault specification is part of a VirtualService rule. Faults include
// aborting the Http request from downstream service, and/or delaying
// proxying of requests. A fault rule MUST HAVE delay or abort or both.
//
// *Note:* Delay and abort faults are independent of one another, even if
// both are specified simultaneously.
type HTTPFaultInjection struct {
	// Delay requests before forwarding, emulating various failures such as
	// network issues, overloaded upstream service, etc.
	Delay *InjectDelay `json:"delay,omitempty"`

	// Abort Http request attempts and return error codes back to downstream
	// service, giving the impression that the upstream service is faulty.
	Abort *InjectAbort `json:"abort,omitempty"`
}

// Delay specification is used to inject latency into the request
// forwarding path. The following example will introduce a 5 second delay
// in 10% of the requests to the "v1" version of the "reviews"
// service from all pods with label env: prod
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: reviews-route
//     spec:
//       hosts:
//       - reviews
//       http:
//       - match:
//         - sourceLabels:
//             env: prod
//         route:
//         - destination:
//             host: reviews
//             subset: v1
//         fault:
//           delay:
//             percent: 10
//             fixedDelay: 5s
//
// The _fixedDelay_ field is used to indicate the amount of delay in
// seconds. An optional _percent_ field, a value between 0 and 100, can
// be used to only delay a certain percentage of requests. If left
// unspecified, all request will be delayed.
type InjectDelay struct {
	// Percentage of requests on which the delay will be injected (0-100).
	Percent int `json:"percent,omitempty"`

	// REQUIRED. Add a fixed delay before forwarding the request. Format:
	// 1h/1m/1s/1ms. MUST be >=1ms.
	FixedDelay string `json:"fixedDelay"`

	// (-- Add a delay (based on an exponential function) before forwarding
	// the request. mean delay needed to derive the exponential delay
	// values --)
	ExponentialDelay string `json:"exponentialDelay,omitempty"`
}

// Abort specification is used to prematurely abort a request with a
// pre-specified error code. The following example will return an HTTP
// 400 error code for 10% of the requests to the "ratings" service "v1".
//
//     apiVersion: networking.istio.io/v1alpha3
//     kind: VirtualService
//     metadata:
//       name: ratings-route
//     spec:
//       hosts:
//       - ratings
//       http:
//       - route:
//         - destination:
//             host: ratings
//             subset: v1
//         fault:
//           abort:
//             percent: 10
//             httpStatus: 400
//
// The _httpStatus_ field is used to indicate the HTTP status code to
// return to the caller. The optional _percent_ field, a value between 0
// and 100, is used to only abort a certain percentage of requests. If
// not specified, all requests are aborted.
type InjectAbort struct {
	// Percentage of requests to be aborted with the error code provided (0-100).
	Percent int `json:"percent,omitempty"`

	// REQUIRED. HTTP status code to use to abort the Http request.
	HTTPStatus int `json:"httpStatus"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualServiceList is a list of VirtualService resources
type VirtualServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []VirtualService `json:"items"`
}
