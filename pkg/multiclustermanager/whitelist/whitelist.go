package whitelist

import (
	"strings"
	"sync"

	"github.com/rancher/rancher/pkg/settings"
)

var (
	Proxy = ProxyAcceptList{
		RWMutex: sync.RWMutex{},
		accept:  map[string]bool{},
	}
)

type ProxyAcceptList struct {
	sync.RWMutex
	accept map[string]bool
}

func (p *ProxyAcceptList) Get() []string {
	p.RLock()
	defer p.RUnlock()
	v := settings.WhitelistDomain.Get()
	r := strings.Split(v, ",")
	for k := range p.accept {
		r = append(r, k)
	}
	return r
}

func (p *ProxyAcceptList) Add(key string) {
	p.Lock()
	defer p.Unlock()
	p.accept[key] = true
}

func (p *ProxyAcceptList) Rm(key string) {
	p.Lock()
	defer p.Unlock()
	delete(p.accept, key)
}
