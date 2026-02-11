package whitelist

import (
	"context"
	"fmt"
	"strings"
	"sync"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	controllersv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

var (
	Proxy = ProxyAcceptList{
		RWMutex:          sync.RWMutex{},
		accept:           map[string]map[string]struct{}{},
		envSettingGetter: settings.WhitelistDomain.Get,
	}
)

type ProxyAcceptList struct {
	sync.RWMutex
	// accept is a mapping between allowed domains
	// and any references to that domain from
	// ProxyEndpoints CRs, Node Driver CRs, and Kontainer drivers.
	accept           map[string]map[string]struct{}
	started          bool
	envSettingGetter func() string
}

// Start registers the onChange and onRemove handlers responsible for watching ProxyEndpoint CRs and updating the
// in-memory domain allow list.
func (p *ProxyAcceptList) Start(ctx context.Context, proxyEndpoint controllersv3.ProxyEndpointController) {
	p.Lock()
	if p.started {
		p.Unlock()
		return
	}
	p.started = true
	p.Unlock()
	proxyEndpoint.OnRemove(ctx, "proxy-accept-list-remover", p.onRemoveEndpoint)
	proxyEndpoint.OnChange(ctx, "proxy-accept-list-adder", p.onChangeEndpoint)
}

func (p *ProxyAcceptList) onRemoveEndpoint(_ string, pe *apimgmtv3.ProxyEndpoint) (*apimgmtv3.ProxyEndpoint, error) {
	if pe == nil || pe.Spec.Routes == nil {
		return pe, nil
	}
	p.RmSource(string(pe.UID))
	return pe, nil
}

func (p *ProxyAcceptList) onChangeEndpoint(_ string, pe *apimgmtv3.ProxyEndpoint) (*apimgmtv3.ProxyEndpoint, error) {
	if pe == nil || pe.Spec.Routes == nil || pe.ObjectMeta.DeletionTimestamp != nil {
		return pe, nil
	}
	// Clear any previous entries for this ProxyEndpoint and then re-add the current list of domains.
	// This is simpler and faster than trying to diff and add/remove the correct domains for existing ProxyEndpoints.
	p.RmSource(string(pe.UID))
	for _, route := range pe.Spec.Routes {
		err := p.Add(route.Domain, string(pe.UID))
		if err != nil {
			logrus.Debugf("failed to add domain %s to whitelist proxy accept list: %v", route.Domain, err)
		}
	}
	return pe, nil
}

// Get returns all domains in the accept list, including those
// defined in the CATTLE_WHITELIST_DOMAIN setting.
func (p *ProxyAcceptList) Get() []string {
	p.RLock()
	defer p.RUnlock()
	envValues := p.envSettingGetter()
	var r []string
	if envValues != "" {
		r = strings.Split(envValues, ",")
	}
	for k := range p.accept {
		r = append(r, k)
	}
	return r
}

// Add adds a domain to the accept list.
// The source parameter is used to track what is adding the domain.
// source should be a unique identifier for the entity adding the domain (e.g., CR UID)
func (p *ProxyAcceptList) Add(key, source string) error {
	p.Lock()
	defer p.Unlock()
	if source == "" {
		return fmt.Errorf("source cannot be empty")
	}
	_, ok := p.accept[key]
	if !ok {
		p.accept[key] = map[string]struct{}{
			source: {},
		}
		return nil
	}
	p.accept[key][source] = struct{}{}
	return nil
}

// Rm removes a domain from the accept list for the given source.
// A domain will only be removed entirely if there are no more sources
// referencing it.
func (p *ProxyAcceptList) Rm(key, source string) {
	if key == "" || source == "" {
		return
	}
	p.Lock()
	defer p.Unlock()

	sources, ok := p.accept[key]
	if !ok {
		logrus.Debugf("domain not found in proxy accept list: %s", key)
		return
	}

	_, present := sources[source]
	if !present {
		return
	}
	delete(sources, source)

	// if there are no more sources for this domain, remove the domain entry
	if len(sources) == 0 {
		delete(p.accept, key)
		return
	}
}

// RmSource removes the provided source from all domains it is associated with.
// If any domain has no more sources after this removal, that domain will be
// removed from the accept list entirely.
func (p *ProxyAcceptList) RmSource(source string) {
	if source == "" {
		return
	}
	p.Lock()
	defer p.Unlock()
	for key, src := range p.accept {
		delete(src, source)
		if len(src) == 0 {
			delete(p.accept, key)
		}
	}
}
