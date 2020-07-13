package ingresshostgen

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/rancher/pkg/controllers/user/approuter"
	"github.com/rancher/rancher/pkg/settings"
	v1beta12 "github.com/rancher/rancher/pkg/types/apis/extensions/v1beta1"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/api/extensions/v1beta1"
)

type IngressHostGen struct {
	ingress v1beta12.IngressInterface
}

func Register(ctx context.Context, userOnlyContext *config.UserOnlyContext) {
	c := &IngressHostGen{
		ingress: userOnlyContext.Extensions.Ingresses(""),
	}
	userOnlyContext.Extensions.Ingresses("").AddHandler(ctx, "ingress-host-gen", c.sync)
}

func isGeneratedDomain(obj *v1beta1.Ingress, host, domain string) bool {
	parts := strings.Split(host, ".")
	return strings.HasSuffix(host, "."+domain) && len(parts) == 8 && parts[1] == obj.Namespace
}

func (i *IngressHostGen) sync(key string, obj *v1beta1.Ingress) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	ipDomain := settings.IngressIPDomain.Get()
	if ipDomain == "" {
		return nil, nil
	}

	var xipHost string
	for _, status := range obj.Status.LoadBalancer.Ingress {
		if status.IP != "" {
			xipHost = fmt.Sprintf("%s.%s.%s.%s", obj.Name, obj.Namespace, status.IP, ipDomain)
			break
		}
	}

	if xipHost == "" {
		return nil, nil
	}

	changed := false
	for _, rule := range obj.Spec.Rules {
		if (isGeneratedDomain(obj, rule.Host, ipDomain) || rule.Host == ipDomain) && rule.Host != xipHost && ipDomain != approuter.RdnsIPDomain {
			changed = true
			break
		}
	}

	if !changed {
		return nil, nil
	}

	obj = obj.DeepCopy()
	for i, rule := range obj.Spec.Rules {
		if strings.HasSuffix(rule.Host, ipDomain) {
			obj.Spec.Rules[i].Host = xipHost
		}
	}

	return i.ingress.Update(obj)
}
