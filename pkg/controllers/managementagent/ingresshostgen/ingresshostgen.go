package ingresshostgen

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/rancher/pkg/ingresswrapper"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
)

type IngressHostGen struct {
	ingress ingresswrapper.CompatInterface
}

func Register(ctx context.Context, userOnlyContext *config.UserOnlyContext) {
	c := &IngressHostGen{
		ingress: ingresswrapper.NewCompatInterface(userOnlyContext.Networking, userOnlyContext.Extensions, userOnlyContext.K8sClient),
	}
	if c.ingress.ServerSupportsIngressV1 {
		userOnlyContext.Networking.Ingresses("").AddHandler(ctx, "ingress-host-gen", ingresswrapper.CompatSyncV1(c.sync))
	} else {
		userOnlyContext.Extensions.Ingresses("").AddHandler(ctx, "ingress-host-gen", ingresswrapper.CompatSyncV1Beta1(c.sync))
	}
}

func isGeneratedDomain(obj ingresswrapper.Ingress, host, domain string) bool {
	parts := strings.Split(host, ".")
	return strings.HasSuffix(host, "."+domain) && len(parts) == 8 && parts[1] == obj.GetNamespace()
}

func (i *IngressHostGen) sync(key string, ingress ingresswrapper.Ingress) (runtime.Object, error) {
	if ingress == nil || reflect.ValueOf(ingress).IsNil() || ingress.GetDeletionTimestamp() != nil {
		return nil, nil
	}

	obj, err := ingresswrapper.ToCompatIngress(ingress)
	if err != nil {
		return obj, err
	}

	ipDomain := settings.IngressIPDomain.Get()
	if ipDomain == "" {
		return nil, nil
	}

	var xipHost string
	for _, status := range obj.Status.LoadBalancer.Ingress {
		if status.IP != "" {
			xipHost = fmt.Sprintf("%s.%s.%s.%s", obj.GetName(), obj.GetNamespace(), status.IP, ipDomain)
			break
		}
	}

	if xipHost == "" {
		return nil, nil
	}

	changed := false
	for _, rule := range obj.Spec.Rules {
		if (isGeneratedDomain(obj, rule.Host, ipDomain) || rule.Host == ipDomain) && rule.Host != xipHost {
			changed = true
			break
		}
	}

	if !changed {
		return nil, nil
	}

	obj, err = ingresswrapper.ToCompatIngress(obj.DeepCopy())
	if err != nil {
		return obj, err
	}
	for i, rule := range obj.Spec.Rules {
		if strings.HasSuffix(rule.Host, ipDomain) {
			obj.Spec.Rules[i].Host = xipHost
		}
	}

	return i.ingress.Update(obj)
}
