package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	PrometheusesGetter
	AlertmanagersGetter
	PrometheusRulesGetter
	ServiceMonitorsGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewFromControllerFactory(factory controller.SharedControllerFactory) (Interface, error) {
	return &Client{
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}, nil
}

type PrometheusesGetter interface {
	Prometheuses(namespace string) PrometheusInterface
}

func (c *Client) Prometheuses(namespace string) PrometheusInterface {
	sharedClient := c.clientFactory.ForResourceKind(PrometheusGroupVersionResource, PrometheusGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PrometheusResource, PrometheusGroupVersionKind, prometheusFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(AlertmanagerGroupVersionResource, AlertmanagerGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &AlertmanagerResource, AlertmanagerGroupVersionKind, alertmanagerFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(PrometheusRuleGroupVersionResource, PrometheusRuleGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PrometheusRuleResource, PrometheusRuleGroupVersionKind, prometheusRuleFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(ServiceMonitorGroupVersionResource, ServiceMonitorGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ServiceMonitorResource, ServiceMonitorGroupVersionKind, serviceMonitorFactory{})
	return &serviceMonitorClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
