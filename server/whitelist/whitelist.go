package whitelist

import (
	"strings"
	"sync"

	"github.com/rancher/rancher/pkg/settings"
)

var (
	Proxy = ProxyList{
		RWMutex:        sync.RWMutex{},
		whitelistProxy: map[string]bool{},
	}
)

type ProxyList struct {
	sync.RWMutex
	whitelistProxy map[string]bool
}

func (p *ProxyList) Get() []string {
	p.RLock()
	defer p.RUnlock()
	v := settings.WhitelistDomain.Get()
	r := strings.Split(v, ",")
	for k := range p.whitelistProxy {
		r = append(r, k)
	}
	return r
}

func (p *ProxyList) Add(key string) {
	p.Lock()
	defer p.Unlock()
	p.whitelistProxy[key] = true
}

func (p *ProxyList) Rm(key string) {
	p.Lock()
	defer p.Unlock()
	delete(p.whitelistProxy, key)
}
