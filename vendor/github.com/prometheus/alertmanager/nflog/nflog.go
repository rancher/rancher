// Copyright 2016 Prometheus Team
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

// Package nflog implements a garbage-collected and snapshottable append-only log of
// active/resolved notifications. Each log entry stores the active/resolved state,
// the notified receiver, and a hash digest of the notification's identifying contents.
// The log can be queried along different paramters.
package nflog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	pb "github.com/prometheus/alertmanager/nflog/nflogpb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/weaveworks/mesh"
)

// ErrNotFound is returned for empty query results.
var ErrNotFound = errors.New("not found")

// Log stores and serves information about notifications
// about byte-slice addressed alert objects to different receivers.
type Log interface {
	// The Log* methods store a notification log entry for
	// a fully qualified receiver and a given IDs identifying the
	// alert object.
	Log(r *pb.Receiver, key string, firing, resolved []uint64) error

	// Query the log along the given Paramteres.
	//
	// TODO(fabxc):
	// - extend the interface by a `QueryOne` method?
	// - return an iterator rather than a materialized list?
	Query(p ...QueryParam) ([]*pb.Entry, error)

	// Snapshot the current log state and return the number
	// of bytes written.
	Snapshot(w io.Writer) (int, error)
	// GC removes expired entries from the log. It returns
	// the total number of deleted entries.
	GC() (int, error)
}

// query currently allows filtering by and/or receiver group key.
// It is configured via QueryParameter functions.
//
// TODO(fabxc): Future versions could allow querying a certain receiver
// group or a given time interval.
type query struct {
	recv     *pb.Receiver
	groupKey string
}

// QueryParam is a function that modifies a query to incorporate
// a set of parameters. Returns an error for invalid or conflicting
// parameters.
type QueryParam func(*query) error

// QReceiver adds a receiver parameter to a query.
func QReceiver(r *pb.Receiver) QueryParam {
	return func(q *query) error {
		q.recv = r
		return nil
	}
}

// QGroupKey adds a group key as querying argument.
func QGroupKey(gk string) QueryParam {
	return func(q *query) error {
		q.groupKey = gk
		return nil
	}
}

type nlog struct {
	logger    log.Logger
	metrics   *metrics
	now       func() time.Time
	retention time.Duration

	runInterval time.Duration
	snapf       string
	stopc       chan struct{}
	done        func()

	gossip mesh.Gossip // gossip channel for sharing log state.

	// For now we only store the most recently added log entry.
	// The key is a serialized concatenation of group key and receiver.
	// Currently our memory state is equivalent to the mesh.GossipData
	// representation. This may change in the future as we support history
	// and indexing.
	mtx sync.RWMutex
	st  gossipData
}

type metrics struct {
	gcDuration       prometheus.Summary
	snapshotDuration prometheus.Summary
	queriesTotal     prometheus.Counter
	queryErrorsTotal prometheus.Counter
	queryDuration    prometheus.Histogram
}

func newMetrics(r prometheus.Registerer) *metrics {
	m := &metrics{}

	m.gcDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "alertmanager_nflog_gc_duration_seconds",
		Help: "Duration of the last notification log garbage collection cycle.",
	})
	m.snapshotDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "alertmanager_nflog_snapshot_duration_seconds",
		Help: "Duration of the last notification log snapshot.",
	})
	m.queriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alertmanager_nflog_queries_total",
		Help: "Number of notification log queries were received.",
	})
	m.queryErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alertmanager_nflog_query_errors_total",
		Help: "Number notification log received queries that failed.",
	})
	m.queryDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "alertmanager_nflog_query_duration_seconds",
		Help: "Duration of notification log query evaluation.",
	})

	if r != nil {
		r.MustRegister(
			m.gcDuration,
			m.snapshotDuration,
			m.queriesTotal,
			m.queryErrorsTotal,
			m.queryDuration,
		)
	}
	return m
}

// Option configures a new Log implementation.
type Option func(*nlog) error

// WithMesh registers the log with a mesh network with which
// the log state will be shared.
func WithMesh(create func(g mesh.Gossiper) mesh.Gossip) Option {
	return func(l *nlog) error {
		l.gossip = create(l)
		return nil
	}
}

// WithRetention sets the retention time for log st.
func WithRetention(d time.Duration) Option {
	return func(l *nlog) error {
		l.retention = d
		return nil
	}
}

// WithNow overwrites the function used to retrieve a timestamp
// for the current point in time.
// This is generally useful for injection during tests.
func WithNow(f func() time.Time) Option {
	return func(l *nlog) error {
		l.now = f
		return nil
	}
}

// WithLogger configures a logger for the notification log.
func WithLogger(logger log.Logger) Option {
	return func(l *nlog) error {
		l.logger = logger
		return nil
	}
}

// WithMetrics registers metrics for the notification log.
func WithMetrics(r prometheus.Registerer) Option {
	return func(l *nlog) error {
		l.metrics = newMetrics(r)
		return nil
	}
}

// WithMaintenance configures the Log to run garbage collection
// and snapshotting, if configured, at the given interval.
//
// The maintenance terminates on receiving from the provided channel.
// The done function is called after the final snapshot was completed.
func WithMaintenance(d time.Duration, stopc chan struct{}, done func()) Option {
	return func(l *nlog) error {
		if d == 0 {
			return fmt.Errorf("maintenance interval must not be 0")
		}
		l.runInterval = d
		l.stopc = stopc
		l.done = done
		return nil
	}
}

// WithSnapshot configures the log to be initialized from a given snapshot file.
// If maintenance is configured, a snapshot will be saved periodically and on
// shutdown as well.
func WithSnapshot(sf string) Option {
	return func(l *nlog) error {
		l.snapf = sf
		return nil
	}
}

func utcNow() time.Time {
	return time.Now().UTC()
}

// New creates a new notification log based on the provided options.
// The snapshot is loaded into the Log if it is set.
func New(opts ...Option) (Log, error) {
	l := &nlog{
		logger: log.NewNopLogger(),
		now:    utcNow,
		st:     map[string]*pb.MeshEntry{},
	}
	for _, o := range opts {
		if err := o(l); err != nil {
			return nil, err
		}
	}
	if l.metrics == nil {
		l.metrics = newMetrics(nil)
	}

	if l.snapf != "" {
		if f, err := os.Open(l.snapf); !os.IsNotExist(err) {
			if err != nil {
				return l, err
			}
			defer f.Close()

			if err := l.loadSnapshot(f); err != nil {
				return l, err
			}
		}
	}

	go l.run()

	return l, nil
}

// run periodic background maintenance.
func (l *nlog) run() {
	if l.runInterval == 0 || l.stopc == nil {
		return
	}
	t := time.NewTicker(l.runInterval)
	defer t.Stop()

	if l.done != nil {
		defer l.done()
	}

	f := func() error {
		start := l.now()
		l.logger.Info("running maintenance")
		defer l.logger.With("duration", l.now().Sub(start)).Info("maintenance done")

		if _, err := l.GC(); err != nil {
			return err
		}
		if l.snapf == "" {
			return nil
		}
		f, err := openReplace(l.snapf)
		if err != nil {
			return err
		}
		// TODO(fabxc): potentially expose snapshot size in log message.
		if _, err := l.Snapshot(f); err != nil {
			return err
		}
		return f.Close()
	}

Loop:
	for {
		select {
		case <-l.stopc:
			break Loop
		case <-t.C:
			if err := f(); err != nil {
				l.logger.With("err", err).Error("running maintenance failed")
			}
		}
	}
	// No need to run final maintenance if we don't want to snapshot.
	if l.snapf == "" {
		return
	}
	if err := f(); err != nil {
		l.logger.With("err", err).Error("creating shutdown snapshot failed")
	}
}

func receiverKey(r *pb.Receiver) string {
	return fmt.Sprintf("%s/%s/%d", r.GroupName, r.Integration, r.Idx)
}

// stateKey returns a string key for a log entry consisting of the group key
// and receiver.
func stateKey(k string, r *pb.Receiver) string {
	return fmt.Sprintf("%s:%s", k, receiverKey(r))
}

func (l *nlog) Log(r *pb.Receiver, gkey string, firingAlerts, resolvedAlerts []uint64) error {
	// Write all st with the same timestamp.
	now := l.now()
	key := stateKey(gkey, r)

	l.mtx.Lock()
	defer l.mtx.Unlock()

	if prevle, ok := l.st[key]; ok {
		// Entry already exists, only overwrite if timestamp is newer.
		// This may happen with raciness or clock-drift across AM nodes.
		if prevle.Entry.Timestamp.After(now) {
			return nil
		}
	}

	e := &pb.MeshEntry{
		Entry: &pb.Entry{
			Receiver:       r,
			GroupKey:       []byte(gkey),
			Timestamp:      now,
			FiringAlerts:   firingAlerts,
			ResolvedAlerts: resolvedAlerts,
		},
		ExpiresAt: now.Add(l.retention),
	}
	l.gossip.GossipBroadcast(gossipData{
		key: e,
	})
	l.st[key] = e

	return nil
}

// GC implements the Log interface.
func (l *nlog) GC() (int, error) {
	start := time.Now()
	defer func() { l.metrics.gcDuration.Observe(time.Since(start).Seconds()) }()

	now := l.now()
	var n int

	l.mtx.Lock()
	defer l.mtx.Unlock()

	for k, le := range l.st {
		if le.ExpiresAt.IsZero() {
			return n, errors.New("unexpected zero expiration timestamp")
		}
		if !le.ExpiresAt.After(now) {
			delete(l.st, k)
			n++
		}
	}

	return n, nil
}

// Query implements the Log interface.
func (l *nlog) Query(params ...QueryParam) ([]*pb.Entry, error) {
	start := time.Now()
	l.metrics.queriesTotal.Inc()

	entries, err := func() ([]*pb.Entry, error) {
		q := &query{}
		for _, p := range params {
			if err := p(q); err != nil {
				return nil, err
			}
		}
		// TODO(fabxc): For now our only query mode is the most recent entry for a
		// receiver/group_key combination.
		if q.recv == nil || q.groupKey == "" {
			// TODO(fabxc): allow more complex queries in the future.
			// How to enable pagination?
			return nil, errors.New("no query parameters specified")
		}

		l.mtx.RLock()
		defer l.mtx.RUnlock()

		if le, ok := l.st[stateKey(q.groupKey, q.recv)]; ok {
			return []*pb.Entry{le.Entry}, nil
		}
		return nil, ErrNotFound
	}()
	if err != nil {
		l.metrics.queryErrorsTotal.Inc()
	}
	l.metrics.queryDuration.Observe(time.Since(start).Seconds())
	return entries, err
}

// loadSnapshot loads a snapshot generated by Snapshot() into the state.
func (l *nlog) loadSnapshot(r io.Reader) error {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	st := gossipData{}

	for {
		var e pb.MeshEntry
		if _, err := pbutil.ReadDelimited(r, &e); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		st[stateKey(string(e.Entry.GroupKey), e.Entry.Receiver)] = &e
	}
	l.st = st

	return nil
}

// Snapshot implements the Log interface.
func (l *nlog) Snapshot(w io.Writer) (int, error) {
	start := time.Now()
	defer func() { l.metrics.snapshotDuration.Observe(time.Since(start).Seconds()) }()

	l.mtx.RLock()
	defer l.mtx.RUnlock()

	var n int
	for _, e := range l.st {
		m, err := pbutil.WriteDelimited(w, e)
		if err != nil {
			return n + m, err
		}
		n += m
	}
	return n, nil
}

// Gossip implements the mesh.Gossiper interface.
func (l *nlog) Gossip() mesh.GossipData {
	l.mtx.RLock()
	defer l.mtx.RUnlock()

	gd := make(gossipData, len(l.st))
	for k, v := range l.st {
		gd[k] = v
	}
	return gd
}

// OnGossip implements the mesh.Gossiper interface.
func (l *nlog) OnGossip(msg []byte) (mesh.GossipData, error) {
	gd, err := decodeGossipData(msg)
	if err != nil {
		return nil, err
	}
	l.mtx.Lock()
	defer l.mtx.Unlock()

	if delta := l.st.mergeDelta(gd); len(delta) > 0 {
		return delta, nil
	}
	return nil, nil
}

// OnGossipBroadcast implements the mesh.Gossiper interface.
func (l *nlog) OnGossipBroadcast(src mesh.PeerName, msg []byte) (mesh.GossipData, error) {
	gd, err := decodeGossipData(msg)
	if err != nil {
		return nil, err
	}
	l.mtx.Lock()
	defer l.mtx.Unlock()

	return l.st.mergeDelta(gd), nil
}

// OnGossipUnicast implements the mesh.Gossiper interface.
func (l *nlog) OnGossipUnicast(src mesh.PeerName, msg []byte) error {
	panic("not implemented")
}

// gossipData is a representation of the current log state that
// implements the mesh.GossipData interface.
type gossipData map[string]*pb.MeshEntry

func decodeGossipData(msg []byte) (gossipData, error) {
	gd := gossipData{}
	rd := bytes.NewReader(msg)

	for {
		var e pb.MeshEntry
		if _, err := pbutil.ReadDelimited(rd, &e); err != nil {
			if err == io.EOF {
				break
			}
			return gd, err
		}
		gd[stateKey(string(e.Entry.GroupKey), e.Entry.Receiver)] = &e
	}

	return gd, nil
}

// Encode implements the mesh.GossipData interface.
func (gd gossipData) Encode() [][]byte {
	// Split into sub-messages of ~1MB.
	const maxSize = 1024 * 1024

	var (
		buf bytes.Buffer
		res [][]byte
		n   int
	)
	for _, e := range gd {
		m, err := pbutil.WriteDelimited(&buf, e)
		n += m
		if err != nil {
			// TODO(fabxc): log error and skip entry. Or can this really not happen with a bytes.Buffer?
			panic(err)
		}
		if n > maxSize {
			res = append(res, buf.Bytes())
			buf = bytes.Buffer{}
		}
	}
	if buf.Len() > 0 {
		res = append(res, buf.Bytes())
	}
	return res
}

func (gd gossipData) clone() gossipData {
	res := make(gossipData, len(gd))
	for k, e := range gd {
		res[k] = e
	}
	return res
}

// Merge the notification set with gossip data and return a new notification
// state.
// TODO(fabxc): can we just return the receiver. Does it have to remain
// unmodified. Needs to be clarified upstream.
func (gd gossipData) Merge(other mesh.GossipData) mesh.GossipData {
	for k, e := range other.(gossipData) {
		prev, ok := gd[k]
		if !ok {
			gd[k] = e
			continue
		}
		if prev.Entry.Timestamp.Before(e.Entry.Timestamp) {
			gd[k] = e
		}
	}
	return gd
}

// mergeDelta behaves like Merge but returns a gossipData only containing
// things that have changed.
func (gd gossipData) mergeDelta(od gossipData) gossipData {
	delta := gossipData{}
	for k, e := range od {
		prev, ok := gd[k]
		if !ok {
			gd[k] = e
			delta[k] = e
			continue
		}
		if prev.Entry.Timestamp.Before(e.Entry.Timestamp) {
			gd[k] = e
			delta[k] = e
		}
	}
	return delta
}

// replaceFile wraps a file that is moved to another filename on closing.
type replaceFile struct {
	*os.File
	filename string
}

func (f *replaceFile) Close() error {
	if err := f.File.Sync(); err != nil {
		return err
	}
	if err := f.File.Close(); err != nil {
		return err
	}
	return os.Rename(f.File.Name(), f.filename)
}

// openReplace opens a new temporary file that is moved to filename on closing.
func openReplace(filename string) (*replaceFile, error) {
	tmpFilename := fmt.Sprintf("%s.%x", filename, uint64(rand.Int63()))

	f, err := os.Create(tmpFilename)
	if err != nil {
		return nil, err
	}

	rf := &replaceFile{
		File:     f,
		filename: filename,
	}
	return rf, nil
}
