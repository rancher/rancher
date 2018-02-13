// Copyright 2015 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package inhibit

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/provider"
	"github.com/prometheus/alertmanager/types"
)

// An Inhibitor determines whether a given label set is muted
// based on the currently active alerts and a set of inhibition rules.
type Inhibitor struct {
	alerts provider.Alerts
	rules  []*InhibitRule
	marker types.Marker

	mtx   sync.RWMutex
	stopc chan struct{}
}

// NewInhibitor returns a new Inhibitor.
func NewInhibitor(ap provider.Alerts, rs []*config.InhibitRule, mk types.Marker) *Inhibitor {
	ih := &Inhibitor{
		alerts: ap,
		marker: mk,
	}
	for _, cr := range rs {
		r := NewInhibitRule(cr)
		ih.rules = append(ih.rules, r)
	}
	return ih
}

func (ih *Inhibitor) runGC() {
	for {
		select {
		case <-time.After(15 * time.Minute):
			for _, r := range ih.rules {
				r.gc()
			}
		case <-ih.stopc:
			return
		}
	}
}

// Run the Inihibitor's background processing.
func (ih *Inhibitor) Run() {
	ih.mtx.Lock()
	ih.stopc = make(chan struct{})
	ih.mtx.Unlock()

	go ih.runGC()

	it := ih.alerts.Subscribe()
	defer it.Close()

	for {
		select {
		case <-ih.stopc:
			return
		case a := <-it.Next():
			if err := it.Err(); err != nil {
				log.Errorf("Error iterating alerts: %s", err)
				continue
			}
			if a.Resolved() {
				// As alerts can also time out without an update, we never
				// handle new resolved alerts but invalidate the cache on read.
				continue
			}
			// Populate the inhibition rules' cache.
			for _, r := range ih.rules {
				if r.SourceMatchers.Match(a.Labels) {
					r.set(a)
				}
			}
		}
	}
}

// Stop the Inhibitor's background processing.
func (ih *Inhibitor) Stop() {
	if ih == nil {
		return
	}
	ih.mtx.Lock()
	defer ih.mtx.Unlock()

	if ih.stopc != nil {
		close(ih.stopc)
		ih.stopc = nil
	}
}

// Mutes returns true iff the given label set is muted.
func (ih *Inhibitor) Mutes(lset model.LabelSet) bool {
	fp := lset.Fingerprint()

	for _, r := range ih.rules {
		if inhibitedByFP, eq := r.hasEqual(lset); r.TargetMatchers.Match(lset) && eq {
			ih.marker.SetInhibited(fp, fmt.Sprintf("%d", inhibitedByFP))
			return true
		}
	}
	ih.marker.SetInhibited(fp)

	return false
}

// An InhibitRule specifies that a class of (source) alerts should inhibit
// notifications for another class of (target) alerts if all specified matching
// labels are equal between the two alerts. This may be used to inhibit alerts
// from sending notifications if their meaning is logically a subset of a
// higher-level alert.
type InhibitRule struct {
	// The set of Filters which define the group of source alerts (which inhibit
	// the target alerts).
	SourceMatchers types.Matchers
	// The set of Filters which define the group of target alerts (which are
	// inhibited by the source alerts).
	TargetMatchers types.Matchers
	// A set of label names whose label values need to be identical in source and
	// target alerts in order for the inhibition to take effect.
	Equal map[model.LabelName]struct{}

	mtx sync.RWMutex
	// Cache of alerts matching source labels.
	scache map[model.Fingerprint]*types.Alert
}

// NewInhibitRule returns a new InihibtRule based on a configuration definition.
func NewInhibitRule(cr *config.InhibitRule) *InhibitRule {
	var (
		sourcem types.Matchers
		targetm types.Matchers
	)

	for ln, lv := range cr.SourceMatch {
		sourcem = append(sourcem, types.NewMatcher(model.LabelName(ln), lv))
	}
	for ln, lv := range cr.SourceMatchRE {
		sourcem = append(sourcem, types.NewRegexMatcher(model.LabelName(ln), lv.Regexp))
	}

	for ln, lv := range cr.TargetMatch {
		targetm = append(targetm, types.NewMatcher(model.LabelName(ln), lv))
	}
	for ln, lv := range cr.TargetMatchRE {
		targetm = append(targetm, types.NewRegexMatcher(model.LabelName(ln), lv.Regexp))
	}

	equal := map[model.LabelName]struct{}{}
	for _, ln := range cr.Equal {
		equal[ln] = struct{}{}
	}

	return &InhibitRule{
		SourceMatchers: sourcem,
		TargetMatchers: targetm,
		Equal:          equal,
		scache:         map[model.Fingerprint]*types.Alert{},
	}
}

// set the alert in the source cache.
func (r *InhibitRule) set(a *types.Alert) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.scache[a.Fingerprint()] = a
}

// hasEqual checks whether the source cache contains alerts matching
// the equal labels for the given label set.
func (r *InhibitRule) hasEqual(lset model.LabelSet) (model.Fingerprint, bool) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

Outer:
	for fp, a := range r.scache {
		// The cache might be stale and contain resolved alerts.
		if a.Resolved() {
			continue
		}
		for n := range r.Equal {
			if a.Labels[n] != lset[n] {
				continue Outer
			}
		}
		return fp, true
	}
	return model.Fingerprint(0), false
}

// gc clears out resolved alerts from the source cache.
func (r *InhibitRule) gc() {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	for fp, a := range r.scache {
		if a.Resolved() {
			delete(r.scache, fp)
		}
	}
}
