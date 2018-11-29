// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	dynamic "k8s.io/client-go/deprecated-dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	Group                 = "monitoring.coreos.com"
	PrometheusKindKey     = "prometheus"
	AlertManagerKindKey   = "alertmanager"
	ServiceMonitorKindKey = "servicemonitor"
	PrometheusRuleKindKey = "prometheusrule"
)

type CrdKind struct {
	Kind     string
	Plural   string
	SpecName string
}

type CrdKinds struct {
	KindsString    string
	Prometheus     CrdKind
	Alertmanager   CrdKind
	ServiceMonitor CrdKind
	PrometheusRule CrdKind
}

var DefaultCrdKinds = CrdKinds{
	KindsString:    "",
	Prometheus:     CrdKind{Plural: PrometheusName, Kind: PrometheusesKind, SpecName: "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1.Prometheus"},
	ServiceMonitor: CrdKind{Plural: ServiceMonitorName, Kind: ServiceMonitorsKind, SpecName: "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1.ServiceMonitor"},
	Alertmanager:   CrdKind{Plural: AlertmanagerName, Kind: AlertmanagersKind, SpecName: "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1.Alertmanager"},
	PrometheusRule: CrdKind{Plural: PrometheusRuleName, Kind: PrometheusRuleKind, SpecName: "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1.PrometheusRule"},
}

// Implement the flag.Value interface
func (crdkinds *CrdKinds) String() string {
	return crdkinds.KindsString
}

// Set Implement the flag.Set interface
func (crdkinds *CrdKinds) Set(value string) error {
	*crdkinds = DefaultCrdKinds
	if value == "" {
		value = fmt.Sprintf("%s=%s:%s,%s=%s:%s,%s=%s:%s,%s=%s:%s",
			PrometheusKindKey, PrometheusesKind, PrometheusName,
			AlertManagerKindKey, AlertmanagersKind, AlertmanagerName,
			ServiceMonitorKindKey, ServiceMonitorsKind, ServiceMonitorName,
			PrometheusRuleKindKey, PrometheusRuleKind, PrometheusRuleName,
		)
	}
	splited := strings.Split(value, ",")
	for _, pair := range splited {
		sp := strings.Split(pair, "=")
		kind := strings.Split(sp[1], ":")
		crdKind := CrdKind{Plural: kind[1], Kind: kind[0]}
		switch kindKey := sp[0]; kindKey {
		case PrometheusKindKey:
			(*crdkinds).Prometheus = crdKind
		case ServiceMonitorKindKey:
			(*crdkinds).ServiceMonitor = crdKind
		case AlertManagerKindKey:
			(*crdkinds).Alertmanager = crdKind
		case PrometheusRuleKindKey:
			(*crdkinds).PrometheusRule = crdKind
		default:
			fmt.Printf("Warning: unknown kind: %s... ignoring", kindKey)
		}

	}
	(*crdkinds).KindsString = value
	return nil
}

var Version = "v1"

type MonitoringV1Interface interface {
	RESTClient() rest.Interface
	PrometheusesGetter
	AlertmanagersGetter
	ServiceMonitorsGetter
	PrometheusRulesGetter
}

// +k8s:deepcopy-gen=false
type MonitoringV1Client struct {
	restClient    rest.Interface
	dynamicClient *dynamic.Client
	crdKinds      *CrdKinds
}

func (c *MonitoringV1Client) Prometheuses(namespace string) PrometheusInterface {
	return newPrometheuses(c.restClient, c.dynamicClient, c.crdKinds.Prometheus, namespace)
}

func (c *MonitoringV1Client) Alertmanagers(namespace string) AlertmanagerInterface {
	return newAlertmanagers(c.restClient, c.dynamicClient, c.crdKinds.Alertmanager, namespace)
}

func (c *MonitoringV1Client) ServiceMonitors(namespace string) ServiceMonitorInterface {
	return newServiceMonitors(c.restClient, c.dynamicClient, c.crdKinds.ServiceMonitor, namespace)
}

func (c *MonitoringV1Client) PrometheusRules(namespace string) PrometheusRuleInterface {
	return newPrometheusRules(c.restClient, c.dynamicClient, c.crdKinds.PrometheusRule, namespace)
}

func (c *MonitoringV1Client) RESTClient() rest.Interface {
	return c.restClient
}

func NewForConfig(crdKinds *CrdKinds, apiGroup string, c *rest.Config) (*MonitoringV1Client, error) {
	config := *c
	SetConfigDefaults(apiGroup, &config)
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewClient(&config, schema.GroupVersion{
		Group:   apiGroup,
		Version: Version,
	})
	if err != nil {
		return nil, err
	}

	return &MonitoringV1Client{client, dynamicClient, crdKinds}, nil
}

func SetConfigDefaults(apiGroup string, config *rest.Config) {
	config.GroupVersion = &schema.GroupVersion{
		Group:   apiGroup,
		Version: Version,
	}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	return
}
