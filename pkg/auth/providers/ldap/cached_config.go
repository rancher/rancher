package ldap

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CachedConfig caches ldap config obtained from another source.
type CachedConfig struct {
	// source is where the real value originates
	source configMapGetter
	// expireAfter is how long to keep the cached value
	expireAfter time.Duration
	// expireAt is the time at which the cache should expire
	expireAt time.Time
	// config is the value being cached
	config map[string]interface{}
}

func NewCachedConfig(source configMapGetter, dur time.Duration) *CachedConfig {
	return &CachedConfig{
		source:      source,
		expireAfter: dur,
		expireAt:    time.Now().Add(dur),
	}
}

func (c *CachedConfig) GetConfigMap(name string, opts metav1.GetOptions) (map[string]interface{}, error) {
	if c.config == nil || time.Now().After(c.expireAt) {
		conf, err := c.source.GetConfigMap(name, opts)
		if err != nil {
			return nil, err
		}
		c.expireAt = time.Now().Add(c.expireAfter)
		c.config = conf
		return conf, nil
	}
	return c.config, nil
}

// Expire invalidates the cache
func (c *CachedConfig) Expire() error {
	c.expireAt = time.Time{}
	c.config = nil
	return nil
}
