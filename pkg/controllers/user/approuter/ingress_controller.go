package approuter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/sirupsen/logrus"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	annotationIngressClass = "kubernetes.io/ingress.class"
	ingressClassNginx      = "nginx"
	RdnsIPDomain           = "lb.rancher.cloud"
	maxHost                = 10
)

var (
	renewInterval = 24 * time.Hour
)

type Controller struct {
	ctx              context.Context
	ingressInterface v1beta1.IngressInterface
	ingressLister    v1beta1.IngressLister
	dnsClient        *Client
}

func isGeneratedDomain(obj *extensionsv1beta1.Ingress, host, domain string) bool {
	parts := strings.Split(host, ".")
	return strings.HasSuffix(host, "."+domain) && len(parts) == 6 && parts[1] == obj.Namespace
}

func (c *Controller) sync(key string, obj *extensionsv1beta1.Ingress) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	ipDomain := settings.IngressIPDomain.Get()
	if ipDomain != RdnsIPDomain {
		return nil, nil
	}

	isNeedSync := false
	for _, rule := range obj.Spec.Rules {
		if strings.HasSuffix(rule.Host, RdnsIPDomain) {
			isNeedSync = true
			break
		}
	}

	if !isNeedSync {
		return nil, nil
	}

	serverURL := settings.RDNSServerBaseURL.Get()
	if serverURL == "" {
		return nil, errors.New("settings.baseRDNSServerURL is not set, dns name might not be reachable")
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
		return nil, err
	}
	//As a new secret is created, all the ingress obj will be updated
	if created {
		return nil, c.refreshAll(fqdn)
	}
	return c.refresh(fqdn, obj)
}

func (c *Controller) refresh(rootDomain string, obj *extensionsv1beta1.Ingress) (*extensionsv1beta1.Ingress, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, errors.New("Got a nil ingress object")
	}

	annotations := obj.Annotations

	if annotations == nil {
		annotations = make(map[string]string)
	}

	targetHostname := ""
	switch annotations[annotationIngressClass] {
	case "": // nginx as default
		fallthrough
	case ingressClassNginx:
		targetHostname = c.getRdnsHostname(obj, rootDomain)
	default:
		return obj, nil
	}
	if targetHostname == "" {
		return obj, nil
	}

	changed := false
	for _, rule := range obj.Spec.Rules {
		if !isGeneratedDomain(obj, rule.Host, rootDomain) {
			changed = true
			break
		}
	}

	if !changed {
		return obj, nil
	}

	newObj := obj.DeepCopy()
	// Also need to update rules for hostname when using nginx
	for i, rule := range newObj.Spec.Rules {
		logrus.Debugf("Got ingress resource hostname: %s", rule.Host)
		if strings.HasSuffix(rule.Host, RdnsIPDomain) {
			newObj.Spec.Rules[i].Host = targetHostname
		}
	}

	if _, err := c.ingressInterface.Update(newObj); err != nil {
		return obj, err
	}

	return newObj, nil
}

func (c *Controller) refreshAll(rootDomain string) error {
	ingresses, err := c.ingressLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	for _, obj := range ingresses {
		if _, err = c.refresh(rootDomain, obj); err != nil {
			logrus.WithError(err).Errorf("refresh ingress %s:%s hostname annotation error", obj.Namespace, obj.Name)
		}
	}
	return nil
}

func (c *Controller) getRdnsHostname(obj *extensionsv1beta1.Ingress, rootDomain string) string {
	if rootDomain != "" {
		return fmt.Sprintf("%s.%s.%s", obj.Name, obj.Namespace, rootDomain)
	}
	return ""
}

func (c *Controller) renew(ctx context.Context) {
	tryTicker := time.NewTicker(renewInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-tryTicker.C:
			ipDomain := settings.IngressIPDomain.Get()
			if ipDomain != RdnsIPDomain {
				continue
			}
			serverURL := settings.RDNSServerBaseURL.Get()
			if serverURL == "" {
				logrus.Warn("RDNSServerBaseURL need to be set when enable approuter controller")
				continue
			}

			c.dnsClient.SetBaseURL(serverURL)

			if fqdn, err := c.dnsClient.RenewDomain(); err != nil {
				logrus.WithError(err).Errorf("renew fqdn [%s] to server [%s] error", fqdn, serverURL)
			}
		}
	}
}
