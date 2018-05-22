package approuter

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/sirupsen/logrus"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

const (
	annotationHostname        = "rdns.cattle.io/hostname"
	annotationIngressClass    = "kubernetes.io/ingress.class"
	ingressClassNginx         = "nginx"
	refreshIngressHostnameKey = "_refreshRDNSHostname_"
	RdnsIPDomain              = "lb.rancher.cloud"
	maxHost                   = 10
)

var (
	renewInterval = 24 * time.Hour
)

type Controller struct {
	ctx                    context.Context
	ingressInterface       v1beta1.IngressInterface
	ingressLister          v1beta1.IngressLister
	managementSecretLister v1.SecretLister
	clusterName            string
	dnsClient              *Client
}

func isGeneratedDomain(obj *extensionsv1beta1.Ingress, host, domain string) bool {
	parts := strings.Split(host, ".")
	return strings.HasSuffix(host, "."+domain) && len(parts) == 6 && parts[1] == obj.Namespace
}

func (c *Controller) sync(key string, obj *extensionsv1beta1.Ingress) error {
	serverURL := settings.RDNSServerBaseURL.Get()
	if serverURL == "" {
		return errors.New("settings.baseRDNSServerURL is not set, dns name might not be reachable")
	}

	var ips []string
	for _, status := range obj.Status.LoadBalancer.Ingress {
		if status.IP != "" {
			ips = append(ips, status.IP)
		}
	}

	if len(ips) > maxHost {
		logrus.Debugf("hosts number is %d, over %d", len(ips), maxHost)
		ips = ips[:maxHost]
	}

	c.dnsClient.SetBaseURL(serverURL)

	created, fqdn, err := c.dnsClient.ApplyDomain(ips)
	if err != nil {
		logrus.WithError(err).Errorf("update fqdn [%s] to server [%s] error", fqdn, serverURL)
		return err
	}
	//As a new secret is created, all the ingress obj will be updated
	if created {
		return c.refreshAll(fqdn)
	}
	return c.refresh(fqdn, obj)
}

func (c *Controller) refresh(rootDomain string, obj *extensionsv1beta1.Ingress) error {
	if obj == nil {
		return errors.New("Got a nil ingress object")
	}

	if obj.ObjectMeta.DeletionTimestamp != nil {
		return nil
	}

	annotations := obj.Annotations

	if annotations == nil {
		annotations = make(map[string]string)
	}

	hostname := annotations[annotationHostname]
	targetHostname := ""
	switch annotations[annotationIngressClass] {
	case "": // nginx as default
		fallthrough
	case ingressClassNginx:
		targetHostname = c.getRdnsHostname(obj, rootDomain)
	default:
		return nil
	}
	if hostname == targetHostname {
		return nil
	}

	ipDomain := settings.IngressIPDomain.Get()
	if ipDomain == "" {
		return nil
	}

	changed := false
	for _, rule := range obj.Spec.Rules {
		if (!isGeneratedDomain(obj, rule.Host, rootDomain) || rule.Host == ipDomain) && ipDomain == RdnsIPDomain {
			changed = true
			break
		}
	}

	if !changed {
		return nil
	}

	newObj := obj.DeepCopy()
	newObj.Annotations[annotationHostname] = targetHostname

	// Also need to update rules for hostname when using nginx
	for i, rule := range newObj.Spec.Rules {
		logrus.Debugf("Got ingress resource hostname: %s", rule.Host)
		if strings.HasSuffix(rule.Host, ipDomain) {
			newObj.Spec.Rules[i].Host = targetHostname
		}
	}

	if _, err := c.ingressInterface.Update(newObj); err != nil {
		return err
	}

	return nil
}

func (c *Controller) refreshAll(rootDomain string) error {
	ingresses, err := c.ingressLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	for _, obj := range ingresses {
		if err = c.refresh(rootDomain, obj); err != nil {
			logrus.WithError(err).Errorf("refresh ingress %s:%s hostname annotation error", obj.Namespace, obj.Name)
		}
	}
	return nil
}

func (c *Controller) getRdnsHostname(obj *extensionsv1beta1.Ingress, rootDomain string) string {
	return fmt.Sprintf("%s.%s.%s", obj.Name, obj.Namespace, rootDomain)
}

func (c *Controller) renew(ctx context.Context) {
	for range ticker.Context(ctx, renewInterval) {
		serverURL := settings.RDNSServerBaseURL.Get()
		if serverURL == "" {
			logrus.Warn("IngressIPDomain and RDNSServerBaseURL need to be set when enable approuter controller")
			return
		}

		c.dnsClient.SetBaseURL(serverURL)

		if fqdn, err := c.dnsClient.RenewDomain(); err != nil {
			logrus.WithError(err).Errorf("renew fqdn [%s] to server [%s] error", fqdn, serverURL)
		}
	}
}
