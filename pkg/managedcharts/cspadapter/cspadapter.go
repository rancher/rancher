// Package cspadapter provides utilities which can help discover if the csp adapter chart is installed,
// for either the original Managed License Offering (MLO) or new Pay-As-You-Go (PAYG) licensing.
package cspadapter

import (
	"errors"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	// MLOChartNamespace is the namespace that we expect the adapter to be installed in,
	// for the original Managed License Offering (MLO) licensing
	MLOChartNamespace = "cattle-csp-adapter-system"
	// MLOChartName is the name of the csp adapter chart for MLO licensing.
	MLOChartName = "rancher-csp-adapter"
	// PAYGChartNamespace is the namespace that we expect the adapter to be installed in,
	// for the new Pay-As-You-Go (PAYG) license offering
	PAYGChartNamespace = "cattle-system"
	// PAYGChartName is the name of the csp adapter chart for PAYG licensing.
	PAYGChartName = "rancher-csp-billing-adapter"
)

// ErrNotFound indicates that the adapter was not found to be installed
var ErrNotFound = errors.New("NotFound")

// ChartUtil provides methods to interact with the cspadapter chart and releases derived form the chart
type ChartUtil struct {
	restClientGetter genericclioptions.RESTClientGetter
}

// NewChartUtil creates a ChartUtil using a RESTClientGetter - this will be used for helm calls to k8s
func NewChartUtil(clientGetter genericclioptions.RESTClientGetter) *ChartUtil {
	return &ChartUtil{
		restClientGetter: clientGetter,
	}
}

// GetRelease finds the release for the CSP adapter for a given offering. If not found, returns nil, ErrNotFound.
func (c *ChartUtil) GetRelease(chartNamespace string, chartName string) (*release.Release, error) {
	cfg := &action.Configuration{}
	if err := cfg.Init(c.restClientGetter, chartNamespace, "", logrus.Infof); err != nil {
		return nil, err
	}
	l := action.NewList(cfg)
	releases, err := l.Run()
	if err != nil {
		return nil, err
	}
	for _, helmRelease := range releases {
		if helmRelease.Chart.Name() == chartName {
			return helmRelease, nil
		}
	}
	return nil, ErrNotFound
}
