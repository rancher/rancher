package plugin

import (
	"sync"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

var (
	Index          = SafeIndex{}
	AnonymousIndex = SafeIndex{}
)

type UIPlugin struct {
	*v1.UIPluginEntry
	CacheState string
	Ready      bool
}

type SafeIndex struct {
	mu      sync.RWMutex
	Entries map[string]*UIPlugin `json:"entries,omitempty"`
}

// Generate generates a new index from a UIPluginCache object
func (s *SafeIndex) Generate(cachedPlugins []*v1.UIPlugin) error {
	logrus.Debug("generating index from plugin controller's cache")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = make(map[string]*UIPlugin, len(cachedPlugins))
	for _, plugin := range cachedPlugins {
		entry := &plugin.Spec.Plugin
		logrus.Debugf("adding plugin to index: %+v", *entry)
		s.Entries[entry.Name] = &UIPlugin{
			UIPluginEntry: entry,
			CacheState:    plugin.Status.CacheState,
			Ready:         plugin.Status.Ready,
		}
	}

	return nil
}

func (s *SafeIndex) Ready(plugin *v1.UIPlugin) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Entries[plugin.Name] != nil {
		s.Entries[plugin.Name].Ready = plugin.Status.Ready
	}
}

func (s *SafeIndex) CacheState(plugin *v1.UIPlugin) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Entries[plugin.Name] != nil {
		s.Entries[plugin.Name].CacheState = plugin.Status.CacheState
	}
}
