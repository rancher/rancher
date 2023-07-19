// Package chart is used for defining helm chart information.
package chart

import (
	"errors"
	"fmt"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"gopkg.in/yaml.v2"
	apierror "k8s.io/apimachinery/pkg/api/errors"
)

const (
	// PriorityClassKey is the name of the helm value used for setting the name of the priority class on pods deployed by rancher and feature charts.
	PriorityClassKey = "priorityClassName"

	// CustomValueMapName is the name of the configMap that hold custom values for charts managed by Rancher.
	CustomValueMapName = "rancher-config"

	// WebhookChartName name of the chart for rancher-webhook.
	WebhookChartName = "rancher-webhook"
)

var errKeyNotFound = errors.New("key not found")

// Manager is an interface used by the handler to install and uninstall charts.
// If the interface is changed, regenerate the mock with the below command (run from project root):
// mockgen --build_flags=--mod=mod -destination=pkg/controllers/dashboard/chart/fake/manager.go -package=fake github.com/rancher/rancher/pkg/controllers/dashboard/chart Manager
type Manager interface {
	// Ensure ensures that the chart is installed into the given namespace with the given version configuration and values.
	Ensure(namespace, name, minVersion, exactVersion string, values map[string]interface{}, forceAdopt bool, installImageOverride string) error

	// Uninstall uninstalls the given chart in the given namespace.
	Uninstall(namespace, name string) error

	// Remove removes the chart from the desired state.
	Remove(namespace, name string)
}

// Definition defines a helm chart.
type Definition struct {
	ReleaseNamespace    string
	ChartName           string
	MinVersionSetting   settings.Setting
	ExactVersionSetting settings.Setting
	Values              func() map[string]interface{}
	Enabled             func() bool
	Uninstall           bool
	RemoveNamespace     bool
}

// RancherConfigGetter is used to get Rancher chart configuration information from the rancher config map.
type RancherConfigGetter struct {
	ConfigCache corev1.ConfigMapCache
}

// GetPriorityClassName attempts to retrieve the priority class for rancher pods and feature charts as set via helm values.
func (r *RancherConfigGetter) GetGlobalValue(key string) (string, error) {
	return r.getKey(key, settings.ConfigMapName.Get())
}

// GetChartValues attempts to retrieve charts values for the specified chart from the rancher-config configMap.
func (r *RancherConfigGetter) GetChartValues(chartName string) (map[string]any, error) {
	strVal, err := r.getKey(chartName, CustomValueMapName)
	if err != nil {
		return nil, err
	}
	retValues := map[string]any{}
	err = yaml.Unmarshal([]byte(strVal), &retValues)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal '%s' value: %w", chartName, err)
	}
	return retValues, nil
}

// getKey attempts to retrieve the provided key for rancher config map.
func (r *RancherConfigGetter) getKey(key, configName string) (string, error) {
	configMap, err := r.ConfigCache.Get(namespace.System, configName)
	if err != nil {
		return "", fmt.Errorf("failed to get ConfigMap '%s': %w", configName, err)
	}
	notFoundErr := fmt.Errorf("'%s' %w in ConfigMap '%s'", key, errKeyNotFound, configMap.Name)
	if configMap.Data == nil {
		return "", notFoundErr
	}
	keyValue, ok := configMap.Data[key]
	if !ok {
		return "", notFoundErr
	}
	return keyValue, nil
}

// IsNotFoundError returns true if the error was caused by either the desired key or ConfigMap not being found.
func IsNotFoundError(err error) bool {
	return apierror.IsNotFound(err) || errors.Is(err, errKeyNotFound)
}
