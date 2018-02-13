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

// Package silence provides a storage for silences, which can share its
// state over a mesh network and snapshot it.
package silence

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	pb "github.com/prometheus/alertmanager/silence/silencepb"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
	"github.com/satori/go.uuid"
	"github.com/weaveworks/mesh"
)

// ErrNotFound is returned if a silence was not found.
var ErrNotFound = fmt.Errorf("not found")

func utcNow() time.Time {
	return time.Now().UTC()
}

type matcherCache map[*pb.Silence]types.Matchers

// Get retrieves the matchers for a given silence. If it is a missed cache
// access, it compiles and adds the matchers of the requested silence to the
// cache.
func (c matcherCache) Get(s *pb.Silence) (types.Matchers, error) {
	if m, ok := c[s]; ok {
		return m, nil
	}
	return c.add(s)
}

// add compiles a silences' matchers and adds them to the cache.
// It returns the compiled matchers.
func (c matcherCache) add(s *pb.Silence) (types.Matchers, error) {
	var (
		ms types.Matchers
		mt *types.Matcher
	)

	for _, m := range s.Matchers {
		mt = &types.Matcher{
			Name:  m.Name,
			Value: m.Pattern,
		}
		switch m.Type {
		case pb.Matcher_EQUAL:
			mt.IsRegex = false
		case pb.Matcher_REGEXP:
			mt.IsRegex = true
		}
		err := mt.Init()
		if err != nil {
			return nil, err
		}

		ms = append(ms, mt)
	}

	c[s] = ms

	return ms, nil
}

// Silences holds a silence state that can be modified, queried, and snapshot.
type Silences struct {
	logger    log.Logger
	metrics   *metrics
	now       func() time.Time
	retention time.Duration

	gossip mesh.Gossip // gossip channel for sharing silences

	// We store silences in a map of IDs for now. Currently, the memory
	// state is equivalent to the mesh.GossipData representation.
	// In the future we'll want support for efficient queries by time
	// range and affected labels.
	// Mutex also guards the matcherCache, which always need write lock access.
	mtx sync.Mutex
	st  *gossipData
	mc  matcherCache
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
		Name: "alertmanager_silences_gc_duration_seconds",
		Help: "Duration of the last silence garbage collection cycle.",
	})
	m.snapshotDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "alertmanager_silences_snapshot_duration_seconds",
		Help: "Duration of the last silence snapshot.",
	})
	m.queriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alertmanager_silences_queries_total",
		Help: "How many silence queries were received.",
	})
	m.queryErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alertmanager_silences_query_errors_total",
		Help: "How many silence received queries did not succeed.",
	})
	m.queryDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "alertmanager_silences_query_duration_seconds",
		Help: "Duration of silence query evaluation.",
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

// Options exposes configuration options for creating a new Silences object.
// Its zero value is a safe default.
type Options struct {
	// A snapshot file or reader from which the initial state is loaded.
	// None or only one of them must be set.
	SnapshotFile   string
	SnapshotReader io.Reader

	// Retention time for newly created Silences. Silences may be
	// garbage collected after the given duration after they ended.
	Retention time.Duration

	// A function creating a mesh.Gossip on being called with a mesh.Gossiper.
	Gossip func(g mesh.Gossiper) mesh.Gossip

	// A logger used by background processing.
	Logger  log.Logger
	Metrics prometheus.Registerer
}

func (o *Options) validate() error {
	if o.SnapshotFile != "" && o.SnapshotReader != nil {
		return fmt.Errorf("only one of SnapshotFile and SnapshotReader must be set")
	}
	return nil
}

// New returns a new Silences object with the given configuration.
func New(o Options) (*Silences, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	if o.SnapshotFile != "" {
		if r, err := os.Open(o.SnapshotFile); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		} else {
			o.SnapshotReader = r
		}
	}
	s := &Silences{
		mc:        matcherCache{},
		logger:    log.NewNopLogger(),
		metrics:   newMetrics(o.Metrics),
		retention: o.Retention,
		now:       utcNow,
		gossip:    nopGossip{},
		st:        newGossipData(),
	}
	if o.Logger != nil {
		s.logger = o.Logger
	}
	if o.Gossip != nil {
		s.gossip = o.Gossip(gossiper{s})
	}
	if o.SnapshotReader != nil {
		if err := s.loadSnapshot(o.SnapshotReader); err != nil {
			return s, err
		}
	}
	return s, nil
}

type nopGossip struct{}

func (nopGossip) GossipBroadcast(d mesh.GossipData)         {}
func (nopGossip) GossipUnicast(mesh.PeerName, []byte) error { return nil }

// Maintenance garbage collects the silence state at the given interval. If the snapshot
// file is set, a snapshot is written to it afterwards.
// Terminates on receiving from stopc.
func (s *Silences) Maintenance(interval time.Duration, snapf string, stopc <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()

	f := func() error {
		start := s.now()
		s.logger.Info("running maintenance")
		defer s.logger.With("duration", s.now().Sub(start)).Info("maintenance done")

		if _, err := s.GC(); err != nil {
			return err
		}
		if snapf == "" {
			return nil
		}
		f, err := openReplace(snapf)
		if err != nil {
			return err
		}
		// TODO(fabxc): potentially expose snapshot size in log message.
		if _, err := s.Snapshot(f); err != nil {
			return err
		}
		return f.Close()
	}

Loop:
	for {
		select {
		case <-stopc:
			break Loop
		case <-t.C:
			if err := f(); err != nil {
				s.logger.With("err", err).Error("running maintenance failed")
			}
		}
	}
	// No need for final maintenance if we don't want to snapshot.
	if snapf == "" {
		return
	}
	if err := f(); err != nil {
		s.logger.With("err", err).Info("msg", "creating shutdown snapshot failed")
	}
}

// GC runs a garbage collection that removes silences that have ended longer
// than the configured retention time ago.
func (s *Silences) GC() (int, error) {
	start := time.Now()
	defer func() { s.metrics.gcDuration.Observe(time.Since(start).Seconds()) }()

	now := s.now()
	var n int

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for id, sil := range s.st.data {
		if sil.ExpiresAt.IsZero() {
			return n, errors.New("unexpected zero expiration timestamp")
		}
		if !sil.ExpiresAt.After(now) {
			delete(s.st.data, id)
			delete(s.mc, sil.Silence)
			n++
		}
	}

	return n, nil
}

func validateMatcher(m *pb.Matcher) error {
	if !model.LabelName(m.Name).IsValid() {
		return fmt.Errorf("invalid label name %q", m.Name)
	}
	switch m.Type {
	case pb.Matcher_EQUAL:
		if !model.LabelValue(m.Pattern).IsValid() {
			return fmt.Errorf("invalid label value %q", m.Pattern)
		}
	case pb.Matcher_REGEXP:
		if _, err := regexp.Compile(m.Pattern); err != nil {
			return fmt.Errorf("invalid regular expression %q: %s", m.Pattern, err)
		}
	default:
		return fmt.Errorf("unknown matcher type %q", m.Type)
	}
	return nil
}

func validateSilence(s *pb.Silence) error {
	if s.Id == "" {
		return errors.New("ID missing")
	}
	if len(s.Matchers) == 0 {
		return errors.New("at least one matcher required")
	}
	for i, m := range s.Matchers {
		if err := validateMatcher(m); err != nil {
			return fmt.Errorf("invalid label matcher %d: %s", i, err)
		}
	}
	if s.StartsAt.IsZero() {
		return errors.New("invalid zero start timestamp")
	}
	if s.EndsAt.IsZero() {
		return errors.New("invalid zero end timestamp")
	}
	if s.EndsAt.Before(s.StartsAt) {
		return errors.New("end time must not be before start time")
	}
	if s.UpdatedAt.IsZero() {
		return errors.New("invalid zero update timestamp")
	}
	return nil
}

// cloneSilence returns a shallow copy of a silence.
func cloneSilence(sil *pb.Silence) *pb.Silence {
	s := *sil
	return &s
}

func (s *Silences) getSilence(id string) (*pb.Silence, bool) {
	msil, ok := s.st.data[id]
	if !ok {
		return nil, false
	}
	return msil.Silence, true
}

func (s *Silences) setSilence(sil *pb.Silence) error {
	sil.UpdatedAt = s.now()

	if err := validateSilence(sil); err != nil {
		return errors.Wrap(err, "silence invalid")
	}

	msil := &pb.MeshSilence{
		Silence:   sil,
		ExpiresAt: sil.EndsAt.Add(s.retention),
	}
	st := &gossipData{
		data: silenceMap{sil.Id: msil},
	}

	s.st.Merge(st)
	// setSilence() is called with s.mtx locked, which can produce
	// a deadlock if we call GossipBroadcast from here.
	go s.gossip.GossipBroadcast(st)

	return nil
}

// Set the specified silence. If a silence with the ID already exists and the modification
// modifies history, the old silence gets expired and a new one is created.
func (s *Silences) Set(sil *pb.Silence) (string, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	now := s.now()
	prev, ok := s.getSilence(sil.Id)

	if sil.Id != "" && !ok {
		return "", ErrNotFound
	}
	if ok {
		if canUpdate(prev, sil, now) {
			return sil.Id, s.setSilence(sil)
		}
		if getState(prev, s.now()) != StateExpired {
			// We cannot update the silence, expire the old one.
			if err := s.expire(prev.Id); err != nil {
				return "", errors.Wrap(err, "expire previous silence")
			}
		}
	}
	// If we got here it's either a new silence or a replacing one.
	sil.Id = uuid.NewV4().String()

	if sil.StartsAt.Before(now) {
		sil.StartsAt = now
	}

	return sil.Id, s.setSilence(sil)
}

// canUpdate returns true if silence a can be updated to b without
// affecting the historic view of silencing.
func canUpdate(a, b *pb.Silence, now time.Time) bool {
	if !reflect.DeepEqual(a.Matchers, b.Matchers) {
		return false
	}
	// Allowed timestamp modifications depend on the current time.
	switch st := getState(a, now); st {
	case StateActive:
		if !b.StartsAt.Equal(a.StartsAt) {
			return false
		}
		if b.EndsAt.Before(now) {
			return false
		}
	case StatePending:
		if b.StartsAt.Before(now) {
			return false
		}
	case StateExpired:
		return false
	default:
		panic("unknown silence state")
	}
	return true
}

// Expire the silence with the given ID immediately.
func (s *Silences) Expire(id string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.expire(id)
}

// Expire the silence with the given ID immediately.
func (s *Silences) expire(id string) error {
	sil, ok := s.getSilence(id)
	if !ok {
		return ErrNotFound
	}
	sil = cloneSilence(sil)
	now := s.now()

	switch getState(sil, now) {
	case StateExpired:
		return errors.Errorf("silence %s already expired", id)
	case StateActive:
		sil.EndsAt = now
	case StatePending:
		// Set both to now to make Silence move to "expired" state
		sil.StartsAt = now
		sil.EndsAt = now
	}

	return s.setSilence(sil)
}

// QueryParam expresses parameters along which silences are queried.
type QueryParam func(*query) error

type query struct {
	ids     []string
	filters []silenceFilter
}

// silenceFilter is a function that returns true if a silence
// should be dropped from a result set for a given time.
type silenceFilter func(*pb.Silence, *Silences, time.Time) (bool, error)

var errNotSupported = errors.New("query parameter not supported")

// QIDs configures a query to select the given silence IDs.
func QIDs(ids ...string) QueryParam {
	return func(q *query) error {
		q.ids = append(q.ids, ids...)
		return nil
	}
}

// QTimeRange configures a query to search for silences that are active
// in the given time range.
// TODO(fabxc): not supported yet.
func QTimeRange(start, end time.Time) QueryParam {
	return func(q *query) error {
		return errNotSupported
	}
}

// QMatches returns silences that match the given label set.
func QMatches(set model.LabelSet) QueryParam {
	return func(q *query) error {
		f := func(sil *pb.Silence, s *Silences, _ time.Time) (bool, error) {
			m, err := s.mc.Get(sil)
			if err != nil {
				return true, err
			}
			return m.Match(set), nil
		}
		q.filters = append(q.filters, f)
		return nil
	}
}

// SilenceState describes the state of a silence based on its time range.
type SilenceState string

// The only possible states of a silence w.r.t a timestamp.
const (
	StateActive  SilenceState = "active"
	StatePending              = "pending"
	StateExpired              = "expired"
)

// getState returns a silence's SilenceState at the given timestamp.
func getState(sil *pb.Silence, ts time.Time) SilenceState {
	if ts.Before(sil.StartsAt) {
		return StatePending
	}
	if ts.After(sil.EndsAt) {
		return StateExpired
	}
	return StateActive
}

// QState filters queried silences by the given states.
func QState(states ...SilenceState) QueryParam {
	return func(q *query) error {
		f := func(sil *pb.Silence, _ *Silences, now time.Time) (bool, error) {
			s := getState(sil, now)

			for _, ps := range states {
				if s == ps {
					return true, nil
				}
			}
			return false, nil
		}
		q.filters = append(q.filters, f)
		return nil
	}
}

// QueryOne queries with the given parameters and returns the first result.
// Returns ErrNotFound if the query result is empty.
func (s *Silences) QueryOne(params ...QueryParam) (*pb.Silence, error) {
	res, err := s.Query(params...)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, ErrNotFound
	}
	return res[0], nil
}

// Query for silences based on the given query parameters.
func (s *Silences) Query(params ...QueryParam) ([]*pb.Silence, error) {
	start := time.Now()
	s.metrics.queriesTotal.Inc()

	sils, err := func() ([]*pb.Silence, error) {
		q := &query{}
		for _, p := range params {
			if err := p(q); err != nil {
				return nil, err
			}
		}
		return s.query(q, s.now())
	}()
	if err != nil {
		s.metrics.queryErrorsTotal.Inc()
	}
	s.metrics.queryDuration.Observe(time.Since(start).Seconds())
	return sils, err
}

func (s *Silences) query(q *query, now time.Time) ([]*pb.Silence, error) {
	// If we have an ID constraint, all silences are our base set.
	// This and the use of post-filter functions is the
	// the trivial solution for now.
	var res []*pb.Silence

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if q.ids != nil {
		for _, id := range q.ids {
			if s, ok := s.st.data[string(id)]; ok {
				res = append(res, s.Silence)
			}
		}
	} else {
		for _, sil := range s.st.data {
			res = append(res, sil.Silence)
		}
	}

	var resf []*pb.Silence
	for _, sil := range res {
		remove := false
		for _, f := range q.filters {
			ok, err := f(sil, s, now)
			if err != nil {
				return nil, err
			}
			if !ok {
				remove = true
				break
			}
		}
		if !remove {
			resf = append(resf, cloneSilence(sil))
		}
	}

	return resf, nil
}

// loadSnapshot loads a snapshot generated by Snapshot() into the state.
// Any previous state is wiped.
func (s *Silences) loadSnapshot(r io.Reader) error {
	st := newGossipData()

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for {
		var sil pb.MeshSilence
		if _, err := pbutil.ReadDelimited(r, &sil); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		// Comments list was moved to a single comment. Upgrade on loading the snapshot.
		if len(sil.Silence.Comments) > 0 {
			sil.Silence.Comment = sil.Silence.Comments[0].Comment
			sil.Silence.CreatedBy = sil.Silence.Comments[0].Author
			sil.Silence.Comments = nil
		}

		st.data[sil.Silence.Id] = &sil
		_, err := s.mc.Get(sil.Silence)
		if err != nil {
			return err
		}
	}

	s.st.data = st.data

	return nil
}

// Snapshot writes the full internal state into the writer and returns the number of bytes
// written.
func (s *Silences) Snapshot(w io.Writer) (int, error) {
	start := time.Now()
	defer func() { s.metrics.snapshotDuration.Observe(time.Since(start).Seconds()) }()

	s.mtx.Lock()
	defer s.mtx.Unlock()

	var n int
	for _, s := range s.st.data {
		m, err := pbutil.WriteDelimited(w, s)
		if err != nil {
			return n + m, err
		}
		n += m
	}
	return n, nil
}

type gossiper struct {
	*Silences
}

// Gossip implements the mesh.Gossiper interface.
func (g gossiper) Gossip() mesh.GossipData {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	return g.st.clone()
}

// OnGossip implements the mesh.Gossiper interface.
func (g gossiper) OnGossip(msg []byte) (mesh.GossipData, error) {
	gd, err := decodeGossipData(msg)
	if err != nil {
		return nil, err
	}
	g.mtx.Lock()
	defer g.mtx.Unlock()

	if delta := g.st.mergeDelta(gd); len(delta.data) > 0 {
		return delta, nil
	}
	return nil, nil
}

// OnGossipBroadcast implements the mesh.Gossiper interface.
func (g gossiper) OnGossipBroadcast(src mesh.PeerName, msg []byte) (mesh.GossipData, error) {
	gd, err := decodeGossipData(msg)
	if err != nil {
		return nil, err
	}
	g.mtx.Lock()
	defer g.mtx.Unlock()

	return g.st.mergeDelta(gd), nil
}

// OnGossipUnicast implements the mesh.Gossiper interface.
// It always panics.
func (g gossiper) OnGossipUnicast(src mesh.PeerName, msg []byte) error {
	panic("not implemented")
}

type silenceMap map[string]*pb.MeshSilence

type gossipData struct {
	data silenceMap
	mtx  sync.RWMutex
}

var _ mesh.GossipData = &gossipData{}

func newGossipData() *gossipData {
	return &gossipData{
		data: silenceMap{},
	}
}

func decodeGossipData(msg []byte) (*gossipData, error) {
	gd := newGossipData()
	rd := bytes.NewReader(msg)

	for {
		var s pb.MeshSilence
		if _, err := pbutil.ReadDelimited(rd, &s); err != nil {
			if err == io.EOF {
				break
			}
			return gd, err
		}
		gd.data[s.Silence.Id] = &s
	}
	return gd, nil
}

// Encode implements the mesh.GossipData interface.
func (gd *gossipData) Encode() [][]byte {
	// Split into sub-messages of ~1MB.
	const maxSize = 1024 * 1024

	var (
		buf bytes.Buffer
		res [][]byte
		n   int
	)

	gd.mtx.RLock()
	defer gd.mtx.RUnlock()

	for _, s := range gd.data {
		m, err := pbutil.WriteDelimited(&buf, s)
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

func (gd *gossipData) clone() *gossipData {
	gd.mtx.RLock()
	defer gd.mtx.RUnlock()

	data := make(silenceMap, len(gd.data))
	for id, s := range gd.data {
		data[id] = s
	}
	return &gossipData{data: data}
}

// Merge the silence set with gossip data and return a new silence state.
func (gd *gossipData) Merge(other mesh.GossipData) mesh.GossipData {
	ot := other.(*gossipData)
	ot.mtx.RLock()
	defer ot.mtx.RUnlock()

	gd.mtx.Lock()
	defer gd.mtx.Unlock()

	for id, s := range ot.data {
		// Comments list was moved to a single comment. Apply upgrade
		// on silences received from peers.
		if len(s.Silence.Comments) > 0 {
			s.Silence.Comment = s.Silence.Comments[0].Comment
			s.Silence.CreatedBy = s.Silence.Comments[0].Author
			s.Silence.Comments = nil
		}

		prev, ok := gd.data[id]
		if !ok {
			gd.data[id] = s
			continue
		}
		if prev.Silence.UpdatedAt.Before(s.Silence.UpdatedAt) {
			gd.data[id] = s
		}
	}
	return gd
}

// mergeDelta behaves like Merge but ignores expired silences, and
// returns a gossipData only containing things that have changed.
func (gd *gossipData) mergeDelta(od *gossipData) *gossipData {
	delta := newGossipData()

	od.mtx.RLock()
	defer od.mtx.RUnlock()

	gd.mtx.Lock()
	defer gd.mtx.Unlock()

	for id, s := range od.data {
		// If a gossiped silence is expired, skip it.
		// For a given silence duration exceeding a few minutes,
		// active silences will have already been gossiped.
		// Once the active silence is gossiped, its expiration
		// should happen more or less simultaneously on the different
		// alertmanager nodes. Preventing the gossiping of expired
		// silences allows them to be GC'd, and doesn't affect
		// consistency across the mesh.
		if !s.ExpiresAt.After(utcNow()) {
			continue
		}

		// Comments list was moved to a single comment. Apply upgrade
		// on silences received from peers.
		if len(s.Silence.Comments) > 0 {
			s.Silence.Comment = s.Silence.Comments[0].Comment
			s.Silence.CreatedBy = s.Silence.Comments[0].Author
			s.Silence.Comments = nil
		}

		prev, ok := gd.data[id]
		if !ok {
			gd.data[id] = s
			delta.data[id] = s
			continue
		}
		if prev.Silence.UpdatedAt.Before(s.Silence.UpdatedAt) {
			gd.data[id] = s
			delta.data[id] = s
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
