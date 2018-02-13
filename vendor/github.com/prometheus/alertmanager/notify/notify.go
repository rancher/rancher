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

package notify

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cespare/xxhash"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
	"golang.org/x/net/context"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/inhibit"
	"github.com/prometheus/alertmanager/nflog"
	"github.com/prometheus/alertmanager/nflog/nflogpb"
	"github.com/prometheus/alertmanager/silence"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
)

var (
	numNotifications = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "alertmanager",
		Name:      "notifications_total",
		Help:      "The total number of attempted notifications.",
	}, []string{"integration"})

	numFailedNotifications = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "alertmanager",
		Name:      "notifications_failed_total",
		Help:      "The total number of failed notifications.",
	}, []string{"integration"})
)

func init() {
	prometheus.Register(numNotifications)
	prometheus.Register(numFailedNotifications)
}

// MinTimeout is the minimum timeout that is set for the context of a call
// to a notification pipeline.
const MinTimeout = 10 * time.Second

// notifyKey defines a custom type with which a context is populated to
// avoid accidental collisions.
type notifyKey int

const (
	keyReceiverName notifyKey = iota
	keyRepeatInterval
	keyGroupLabels
	keyGroupKey
	keyFiringAlerts
	keyResolvedAlerts
	keyNow
)

// WithReceiverName populates a context with a receiver name.
func WithReceiverName(ctx context.Context, rcv string) context.Context {
	return context.WithValue(ctx, keyReceiverName, rcv)
}

// WithGroupKey populates a context with a group key.
func WithGroupKey(ctx context.Context, s string) context.Context {
	return context.WithValue(ctx, keyGroupKey, s)
}

// WithFiringAlerts populates a context with a slice of firing alerts.
func WithFiringAlerts(ctx context.Context, alerts []uint64) context.Context {
	return context.WithValue(ctx, keyFiringAlerts, alerts)
}

// WithResolvedAlerts populates a context with a slice of resolved alerts.
func WithResolvedAlerts(ctx context.Context, alerts []uint64) context.Context {
	return context.WithValue(ctx, keyResolvedAlerts, alerts)
}

// WithGroupLabels populates a context with grouping labels.
func WithGroupLabels(ctx context.Context, lset model.LabelSet) context.Context {
	return context.WithValue(ctx, keyGroupLabels, lset)
}

// WithNow populates a context with a now timestamp.
func WithNow(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, keyNow, t)
}

// WithRepeatInterval populates a context with a repeat interval.
func WithRepeatInterval(ctx context.Context, t time.Duration) context.Context {
	return context.WithValue(ctx, keyRepeatInterval, t)
}

// RepeatInterval extracts a repeat interval from the context. Iff none exists, the
// second argument is false.
func RepeatInterval(ctx context.Context) (time.Duration, bool) {
	v, ok := ctx.Value(keyRepeatInterval).(time.Duration)
	return v, ok
}

// ReceiverName extracts a receiver name from the context. Iff none exists, the
// second argument is false.
func ReceiverName(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyReceiverName).(string)
	return v, ok
}

func receiverName(ctx context.Context) string {
	recv, ok := ReceiverName(ctx)
	if !ok {
		log.Error("missing receiver")
	}
	return recv
}

// GroupKey extracts a group key from the context. Iff none exists, the
// second argument is false.
func GroupKey(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyGroupKey).(string)
	return v, ok
}

func groupLabels(ctx context.Context) model.LabelSet {
	groupLabels, ok := GroupLabels(ctx)
	if !ok {
		log.Error("missing group labels")
	}
	return groupLabels
}

// GroupLabels extracts grouping label set from the context. Iff none exists, the
// second argument is false.
func GroupLabels(ctx context.Context) (model.LabelSet, bool) {
	v, ok := ctx.Value(keyGroupLabels).(model.LabelSet)
	return v, ok
}

// Now extracts a now timestamp from the context. Iff none exists, the
// second argument is false.
func Now(ctx context.Context) (time.Time, bool) {
	v, ok := ctx.Value(keyNow).(time.Time)
	return v, ok
}

// FiringAlerts extracts a slice of firing alerts from the context.
// Iff none exists, the second argument is false.
func FiringAlerts(ctx context.Context) ([]uint64, bool) {
	v, ok := ctx.Value(keyFiringAlerts).([]uint64)
	return v, ok
}

// ResolvedAlerts extracts a slice of firing alerts from the context.
// Iff none exists, the second argument is false.
func ResolvedAlerts(ctx context.Context) ([]uint64, bool) {
	v, ok := ctx.Value(keyResolvedAlerts).([]uint64)
	return v, ok
}

// A Stage processes alerts under the constraints of the given context.
type Stage interface {
	Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error)
}

// StageFunc wraps a function to represent a Stage.
type StageFunc func(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error)

// Exec implements Stage interface.
func (f StageFunc) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	return f(ctx, alerts...)
}

// BuildPipeline builds a map of receivers to Stages.
func BuildPipeline(
	confs []*config.Receiver,
	tmpl *template.Template,
	wait func() time.Duration,
	inhibitor *inhibit.Inhibitor,
	silences *silence.Silences,
	notificationLog nflog.Log,
	marker types.Marker,
) RoutingStage {
	rs := RoutingStage{}

	is := NewInhibitStage(inhibitor, marker)
	ss := NewSilenceStage(silences, marker)

	for _, rc := range confs {
		rs[rc.Name] = MultiStage{is, ss, createStage(rc, tmpl, wait, notificationLog)}
	}
	return rs
}

// createStage creates a pipeline of stages for a receiver.
func createStage(rc *config.Receiver, tmpl *template.Template, wait func() time.Duration, notificationLog nflog.Log) Stage {
	var fs FanoutStage
	for _, i := range BuildReceiverIntegrations(rc, tmpl) {
		recv := &nflogpb.Receiver{
			GroupName:   rc.Name,
			Integration: i.name,
			Idx:         uint32(i.idx),
		}
		var s MultiStage
		s = append(s, NewWaitStage(wait))
		s = append(s, NewDedupStage(notificationLog, recv, i.conf.SendResolved()))
		s = append(s, NewRetryStage(i))
		s = append(s, NewSetNotifiesStage(notificationLog, recv))

		fs = append(fs, s)
	}
	return fs
}

// RoutingStage executes the inner stages based on the receiver specified in
// the context.
type RoutingStage map[string]Stage

// Exec implements the Stage interface.
func (rs RoutingStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	receiver, ok := ReceiverName(ctx)
	if !ok {
		return ctx, nil, fmt.Errorf("receiver missing")
	}

	s, ok := rs[receiver]
	if !ok {
		return ctx, nil, fmt.Errorf("stage for receiver missing")
	}

	return s.Exec(ctx, alerts...)
}

// A MultiStage executes a series of stages sequencially.
type MultiStage []Stage

// Exec implements the Stage interface.
func (ms MultiStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	var err error
	for _, s := range ms {
		if len(alerts) == 0 {
			return ctx, nil, nil
		}

		ctx, alerts, err = s.Exec(ctx, alerts...)
		if err != nil {
			return ctx, nil, err
		}
	}
	return ctx, alerts, nil
}

// FanoutStage executes its stages concurrently
type FanoutStage []Stage

// Exec attempts to execute all stages concurrently and discards the results.
// It returns its input alerts and a types.MultiError if one or more stages fail.
func (fs FanoutStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	var (
		wg sync.WaitGroup
		me types.MultiError
	)
	wg.Add(len(fs))

	for _, s := range fs {
		go func(s Stage) {
			if _, _, err := s.Exec(ctx, alerts...); err != nil {
				me.Add(err)
				log.Errorf("Error on notify: %s", err)
			}
			wg.Done()
		}(s)
	}
	wg.Wait()

	if me.Len() > 0 {
		return ctx, alerts, &me
	}
	return ctx, alerts, nil
}

// InhibitStage filters alerts through an inhibition muter.
type InhibitStage struct {
	muter  types.Muter
	marker types.Marker
}

// NewInhibitStage return a new InhibitStage.
func NewInhibitStage(m types.Muter, mk types.Marker) *InhibitStage {
	return &InhibitStage{
		muter:  m,
		marker: mk,
	}
}

// Exec implements the Stage interface.
func (n *InhibitStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	var filtered []*types.Alert
	for _, a := range alerts {
		_, ok := n.marker.Inhibited(a.Fingerprint())
		// TODO(fabxc): increment total alerts counter.
		// Do not send the alert if the silencer mutes it.
		if !n.muter.Mutes(a.Labels) {
			// TODO(fabxc): increment muted alerts counter.
			filtered = append(filtered, a)
			// Store whether a previously inhibited alert is firing again.
			a.WasInhibited = ok
		}
	}

	return ctx, filtered, nil
}

// SilenceStage filters alerts through a silence muter.
type SilenceStage struct {
	silences *silence.Silences
	marker   types.Marker
}

// NewSilenceStage returns a new SilenceStage.
func NewSilenceStage(s *silence.Silences, mk types.Marker) *SilenceStage {
	return &SilenceStage{
		silences: s,
		marker:   mk,
	}
}

// Exec implements the Stage interface.
func (n *SilenceStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	var filtered []*types.Alert
	for _, a := range alerts {
		_, ok := n.marker.Silenced(a.Fingerprint())
		// TODO(fabxc): increment total alerts counter.
		// Do not send the alert if the silencer mutes it.
		sils, err := n.silences.Query(
			silence.QState(silence.StateActive),
			silence.QMatches(a.Labels),
		)
		if err != nil {
			log.Errorf("Querying silences failed: %s", err)
		}

		if len(sils) == 0 {
			// TODO(fabxc): increment muted alerts counter.
			filtered = append(filtered, a)
			n.marker.SetSilenced(a.Labels.Fingerprint())
			// Store whether a previously silenced alert is firing again.
			a.WasSilenced = ok
		} else {
			ids := make([]string, len(sils))
			for i, s := range sils {
				ids[i] = s.Id
			}
			n.marker.SetSilenced(a.Labels.Fingerprint(), ids...)
		}
	}

	return ctx, filtered, nil
}

// WaitStage waits for a certain amount of time before continuing or until the
// context is done.
type WaitStage struct {
	wait func() time.Duration
}

// NewWaitStage returns a new WaitStage.
func NewWaitStage(wait func() time.Duration) *WaitStage {
	return &WaitStage{
		wait: wait,
	}
}

// Exec implements the Stage interface.
func (ws *WaitStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	select {
	case <-time.After(ws.wait()):
	case <-ctx.Done():
		return ctx, nil, ctx.Err()
	}
	return ctx, alerts, nil
}

// DedupStage filters alerts.
// Filtering happens based on a notification log.
type DedupStage struct {
	nflog        nflog.Log
	recv         *nflogpb.Receiver
	sendResolved bool

	now  func() time.Time
	hash func(*types.Alert) uint64
}

// NewDedupStage wraps a DedupStage that runs against the given notification log.
func NewDedupStage(l nflog.Log, recv *nflogpb.Receiver, sendResolved bool) *DedupStage {
	return &DedupStage{
		nflog:        l,
		recv:         recv,
		now:          utcNow,
		sendResolved: sendResolved,
		hash:         hashAlert,
	}
}

func utcNow() time.Time {
	return time.Now().UTC()
}

var hashBuffers = sync.Pool{}

func getHashBuffer() []byte {
	b := hashBuffers.Get()
	if b == nil {
		return make([]byte, 0, 1024)
	}
	return b.([]byte)
}

func putHashBuffer(b []byte) {
	b = b[:0]
	hashBuffers.Put(b)
}

func hashAlert(a *types.Alert) uint64 {
	const sep = '\xff'

	b := getHashBuffer()
	defer putHashBuffer(b)

	names := make(model.LabelNames, 0, len(a.Labels))

	for ln, _ := range a.Labels {
		names = append(names, ln)
	}
	sort.Sort(names)

	for _, ln := range names {
		b = append(b, string(ln)...)
		b = append(b, sep)
		b = append(b, string(a.Labels[ln])...)
		b = append(b, sep)
	}

	hash := xxhash.Sum64(b)

	return hash
}

func allAlertsResolved(alerts []*types.Alert) bool {
	for _, a := range alerts {
		if !a.Resolved() {
			return false
		}
	}
	return true
}

func (n *DedupStage) needsUpdate(entry *nflogpb.Entry, firing, resolved map[uint64]struct{}, repeat time.Duration) (bool, error) {
	// If we haven't notified about the alert group before, notify right away
	// unless we only have resolved alerts.
	if entry == nil {
		return len(firing) > 0, nil
	}

	if !entry.IsFiringSubset(firing) {
		return true, nil
	}

	if n.sendResolved && !entry.IsResolvedSubset(resolved) {
		return true, nil
	}

	// Nothing changed, only notify if the repeat interval has passed.
	return entry.Timestamp.Before(n.now().Add(-repeat)), nil
}

// Exec implements the Stage interface.
func (n *DedupStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	gkey, ok := GroupKey(ctx)
	if !ok {
		return ctx, nil, fmt.Errorf("group key missing")
	}

	repeatInterval, ok := RepeatInterval(ctx)
	if !ok {
		return ctx, nil, fmt.Errorf("repeat interval missing")
	}

	firingSet := map[uint64]struct{}{}
	resolvedSet := map[uint64]struct{}{}
	firing := []uint64{}
	resolved := []uint64{}

	var hash uint64
	for _, a := range alerts {
		hash = n.hash(a)
		if a.Resolved() {
			resolved = append(resolved, hash)
			resolvedSet[hash] = struct{}{}
		} else {
			firing = append(firing, hash)
			firingSet[hash] = struct{}{}
		}
	}

	ctx = WithFiringAlerts(ctx, firing)
	ctx = WithResolvedAlerts(ctx, resolved)

	entries, err := n.nflog.Query(nflog.QGroupKey(gkey), nflog.QReceiver(n.recv))

	if err != nil && err != nflog.ErrNotFound {
		return ctx, nil, err
	}
	var entry *nflogpb.Entry
	switch len(entries) {
	case 0:
	case 1:
		entry = entries[0]
	case 2:
		return ctx, nil, fmt.Errorf("Unexpected entry result size %d", len(entries))
	}
	if ok, err := n.needsUpdate(entry, firingSet, resolvedSet, repeatInterval); err != nil {
		return ctx, nil, err
	} else if ok {
		return ctx, alerts, nil
	}
	return ctx, nil, nil
}

// RetryStage notifies via passed integration with exponential backoff until it
// succeeds. It aborts if the context is canceled or timed out.
type RetryStage struct {
	integration Integration
}

// NewRetryStage returns a new instance of a RetryStage.
func NewRetryStage(i Integration) *RetryStage {
	return &RetryStage{
		integration: i,
	}
}

// Exec implements the Stage interface.
func (r RetryStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	var (
		i    = 0
		b    = backoff.NewExponentialBackOff()
		tick = backoff.NewTicker(b)
		iErr error
	)
	defer tick.Stop()

	for {
		i++
		// Always check the context first to not notify again.
		select {
		case <-ctx.Done():
			if iErr != nil {
				return ctx, nil, iErr
			}

			return ctx, nil, ctx.Err()
		default:
		}

		select {
		case <-tick.C:
			if retry, err := r.integration.Notify(ctx, alerts...); err != nil {
				numFailedNotifications.WithLabelValues(r.integration.name).Inc()
				log.Debugf("Notify attempt %d for %q failed: %s", i, r.integration.name, err)
				if !retry {
					return ctx, alerts, fmt.Errorf("Cancelling notify retry for %q due to unrecoverable error: %s", r.integration.name, err)
				}

				// Save this error to be able to return the last seen error by an
				// integration upon context timeout.
				iErr = err
			} else {
				numNotifications.WithLabelValues(r.integration.name).Inc()
				return ctx, alerts, nil
			}
		case <-ctx.Done():
			if iErr != nil {
				return ctx, nil, iErr
			}

			return ctx, nil, ctx.Err()
		}
	}
}

// SetNotifiesStage sets the notification information about passed alerts. The
// passed alerts should have already been sent to the receivers.
type SetNotifiesStage struct {
	nflog nflog.Log
	recv  *nflogpb.Receiver
}

// NewSetNotifiesStage returns a new instance of a SetNotifiesStage.
func NewSetNotifiesStage(l nflog.Log, recv *nflogpb.Receiver) *SetNotifiesStage {
	return &SetNotifiesStage{
		nflog: l,
		recv:  recv,
	}
}

// Exec implements the Stage interface.
func (n SetNotifiesStage) Exec(ctx context.Context, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	gkey, ok := GroupKey(ctx)
	if !ok {
		return ctx, nil, fmt.Errorf("group key missing")
	}

	firing, ok := FiringAlerts(ctx)
	if !ok {
		return ctx, nil, fmt.Errorf("firing alerts missing")
	}

	resolved, ok := ResolvedAlerts(ctx)
	if !ok {
		return ctx, nil, fmt.Errorf("resolved alerts missing")
	}

	return ctx, alerts, n.nflog.Log(n.recv, gkey, firing, resolved)
}
