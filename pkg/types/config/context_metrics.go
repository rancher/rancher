package config

import (
	"fmt"
	"io"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	downstreamTotalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "downstream_k8s",
			Name:      "total_requests",
			Help:      "Total requests to downstream Kubernetes APIs by path",
		},
		[]string{"context_cluster_name", "method", "path", "watch"})
	downstreamRecBytesByPath = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "downstream_k8s",
			Name:      "received_bytes",
			Help:      "Bytes received from downstream Kubernetes APIs by path",
		},
		[]string{"context_cluster_name", "method", "path", "watch"})
	downstreamSentBytesByPath = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "downstream_k8s",
			Name:      "sent_bytes",
			Help:      "Bytes sent to downstream Kubernetes APIs by path",
		},
		[]string{"context_cluster_name", "method", "path", "watch"})
)

func RegisterDownstreamK8sMetrics(registerer prometheus.Registerer) {
	registerer.MustRegister(
		downstreamTotalRequests,
		downstreamSentBytesByPath,
		downstreamRecBytesByPath,
	)
}

type kindMetricsRoundTripper struct {
	clusterName string
	wrapped     http.RoundTripper
}

func isWatch(req *http.Request) bool {
	// List vs. Watch requests are both GET request, but the latter sets a "watch" query parameter
	return req.URL.Query().Has("watch")
}

func (k kindMetricsRoundTripper) interceptRequest(req *http.Request) {
	downstreamTotalRequests.WithLabelValues(k.clusterName, req.Method, req.URL.Path, fmt.Sprintf("%v", isWatch(req))).Inc()
	if req.Body == nil {
		return
	}
	wrapper := &respBodyWrapper{req.Body,
		func(n int) {
			downstreamSentBytesByPath.WithLabelValues(k.clusterName, req.Method, req.URL.Path, fmt.Sprintf("%v", isWatch(req))).Add(float64(n))
		}}
	req.Body = wrapper
}

func (k kindMetricsRoundTripper) interceptResponse(resp *http.Response) {
	if resp.Body == nil {
		return
	}
	wrapper := &respBodyWrapper{resp.Body,
		func(n int) {
			downstreamRecBytesByPath.WithLabelValues(k.clusterName, resp.Request.Method, resp.Request.URL.Path, fmt.Sprintf("%v", isWatch(resp.Request))).Add(float64(n))
		}}
	resp.Body = wrapper
}

// RoundTrip implements the http.RoundTripper interface. It allows intercepting requests and responses
func (k kindMetricsRoundTripper) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	k.interceptRequest(req)
	defer func() {
		if err == nil && resp != nil {
			k.interceptResponse(resp)
		}
	}()
	return k.wrapped.RoundTrip(req)
}

// respBodyWrapper wraps an io.ReadCloser's Read method to execute a callback with the number of read bytes
type respBodyWrapper struct {
	io.ReadCloser
	record func(int)
}

func (r respBodyWrapper) Read(p []byte) (n int, err error) {
	defer func() {
		r.record(n)
	}()
	return r.ReadCloser.Read(p)
}
