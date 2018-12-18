package v1

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/rest"
)

type (
	contextKeyType        struct{}
	contextClientsKeyType struct{}
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	PrometheusesGetter
	AlertmanagersGetter
	PrometheusRulesGetter
	ServiceMonitorsGetter
}

type Clients struct {
	Interface Interface

	Prometheus     PrometheusClient
	Alertmanager   AlertmanagerClient
	PrometheusRule PrometheusRuleClient
	ServiceMonitor ServiceMonitorClient
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	prometheusControllers     map[string]PrometheusController
	alertmanagerControllers   map[string]AlertmanagerController
	prometheusRuleControllers map[string]PrometheusRuleController
	serviceMonitorControllers map[string]ServiceMonitorController
}

func Factory(ctx context.Context, config rest.Config) (context.Context, controller.Starter, error) {
	c, err := NewForConfig(config)
	if err != nil {
		return ctx, nil, err
	}

	cs := NewClientsFromInterface(c)

	ctx = context.WithValue(ctx, contextKeyType{}, c)
	ctx = context.WithValue(ctx, contextClientsKeyType{}, cs)
	return ctx, c, nil
}

func ClientsFrom(ctx context.Context) *Clients {
	return ctx.Value(contextClientsKeyType{}).(*Clients)
}

func From(ctx context.Context) Interface {
	return ctx.Value(contextKeyType{}).(Interface)
}

func NewClients(config rest.Config) (*Clients, error) {
	iface, err := NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewClientsFromInterface(iface), nil
}

func NewClientsFromInterface(iface Interface) *Clients {
	return &Clients{
		Interface: iface,

		Prometheus: &prometheusClient2{
			iface: iface.Prometheuses(""),
		},
		Alertmanager: &alertmanagerClient2{
			iface: iface.Alertmanagers(""),
		},
		PrometheusRule: &prometheusRuleClient2{
			iface: iface.PrometheusRules(""),
		},
		ServiceMonitor: &serviceMonitorClient2{
			iface: iface.ServiceMonitors(""),
		},
	}
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	restClient, err := restwatch.UnversionedRESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,

		prometheusControllers:     map[string]PrometheusController{},
		alertmanagerControllers:   map[string]AlertmanagerController{},
		prometheusRuleControllers: map[string]PrometheusRuleController{},
		serviceMonitorControllers: map[string]ServiceMonitorController{},
	}, nil
}

func (c *Client) RESTClient() rest.Interface {
	return c.restClient
}

func (c *Client) Sync(ctx context.Context) error {
	return controller.Sync(ctx, c.starters...)
}

func (c *Client) Start(ctx context.Context, threadiness int) error {
	return controller.Start(ctx, threadiness, c.starters...)
}

type PrometheusesGetter interface {
	Prometheuses(namespace string) PrometheusInterface
}

func (c *Client) Prometheuses(namespace string) PrometheusInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PrometheusResource, PrometheusGroupVersionKind, prometheusFactory{})
	return &prometheusClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type AlertmanagersGetter interface {
	Alertmanagers(namespace string) AlertmanagerInterface
}

func (c *Client) Alertmanagers(namespace string) AlertmanagerInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &AlertmanagerResource, AlertmanagerGroupVersionKind, alertmanagerFactory{})
	return &alertmanagerClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PrometheusRulesGetter interface {
	PrometheusRules(namespace string) PrometheusRuleInterface
}

func (c *Client) PrometheusRules(namespace string) PrometheusRuleInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PrometheusRuleResource, PrometheusRuleGroupVersionKind, prometheusRuleFactory{})
	return &prometheusRuleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ServiceMonitorsGetter interface {
	ServiceMonitors(namespace string) ServiceMonitorInterface
}

func (c *Client) ServiceMonitors(namespace string) ServiceMonitorInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ServiceMonitorResource, ServiceMonitorGroupVersionKind, serviceMonitorFactory{})
	return &serviceMonitorClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
