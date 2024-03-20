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

type SafeIndex struct {
	mu      sync.RWMutex
	Entries map[string]*v1.UIPluginEntry `json:"entries,omitempty"`
}

// Generate generates a new index from a UIPluginCache object
func (s *SafeIndex) Generate(cachedPlugins []*v1.UIPlugin) error {
	logrus.Debug("generating index from plugin controller's cache")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = make(map[string]*v1.UIPluginEntry, len(cachedPlugins))
	for _, plugin := range cachedPlugins {
		entry := &plugin.Spec.Plugin
		logrus.Debugf("adding plugin to index: %+v", *entry)
		s.Entries[entry.Name] = entry
	}

	return nil
}
