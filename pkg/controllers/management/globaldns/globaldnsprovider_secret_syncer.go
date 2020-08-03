package globaldns

import (
	"context"

	"strings"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	GlobaldnsProviderSecretSyncer = "mgmt-global-dns-provider-secret-syncer"
)

type ProviderSecretSyncer struct {
	globalDNSProviders      v3.GlobalDNSProviderInterface
	globalDNSProviderLister v3.GlobalDNSProviderLister
}

func newProviderSecretSyncer(ctx context.Context, mgmt *config.ManagementContext) *ProviderSecretSyncer {
	n := &ProviderSecretSyncer{
		globalDNSProviders:      mgmt.Management.GlobalDNSProviders(""),
		globalDNSProviderLister: mgmt.Management.GlobalDNSProviders("").Controller().Lister(),
	}
	return n
}

//sync is called periodically and on real updates
func (n *ProviderSecretSyncer) sync(key string, obj *corev1.Secret) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if !strings.EqualFold(obj.Namespace, namespace.GlobalNamespace) {
		return nil, nil
	}

	return n.reconcileAllGlobalDNSProviders(key)
}

func (n *ProviderSecretSyncer) reconcileAllGlobalDNSProviders(key string) (runtime.Object, error) {
	globalDNSProviderList, err := n.globalDNSProviderLister.List(namespace.GlobalNamespace, labels.NewSelector())
	if err != nil {
		return nil, err
	}

	for _, globalDNSProviderObj := range globalDNSProviderList {
		//call update on each GlobalDNSProvider obj that refers to this current secret
		usesSecret := n.doesGlobalDNSProviderUseSecret(key, globalDNSProviderObj)
		if usesSecret {
			//enqueue it to the globalDNS controller
			n.globalDNSProviders.Controller().Enqueue(namespace.GlobalNamespace, globalDNSProviderObj.Name)
		}
	}
	return nil, nil
}

func (n *ProviderSecretSyncer) doesGlobalDNSProviderUseSecret(key string, globalDNSProviderObj *v3.GlobalDNSProvider) bool {
	var secretRef string
	if globalDNSProviderObj.Spec.Route53ProviderConfig != nil {
		secretRef = globalDNSProviderObj.Spec.Route53ProviderConfig.SecretKey
	}

	if globalDNSProviderObj.Spec.CloudflareProviderConfig != nil {
		secretRef = globalDNSProviderObj.Spec.CloudflareProviderConfig.APIKey
	}

	if globalDNSProviderObj.Spec.AlidnsProviderConfig != nil {
		secretRef = globalDNSProviderObj.Spec.AlidnsProviderConfig.SecretKey
	}

	if secretRef != "" && strings.HasPrefix(secretRef, namespace.GlobalNamespace) {
		ns, secret := ref.Parse(secretRef)
		if strings.EqualFold(key, ns+"/"+secret) {
			return true
		}

	}
	return false
}
