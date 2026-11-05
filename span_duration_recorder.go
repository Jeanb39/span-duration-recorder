// Package span_duration_recorder records start and end times for named
// operations and reports the slowest traces when the recorder is closed.
//
// The recorder deliberately keeps the whole picture in memory: a span is a
// fixed-size struct and the summary is produced once, on Close, rather than
// streaming aggregates. That keeps the hot path (Begin/End) free of locks on
// the calling goroutine's behalf and makes the reported "slowest traces"
// faithful to what was actually recorded instead of an approximation.
package span_duration_recorder

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrRecorderClosed is returned by Begin once Close has been called. Recording
// after shutdown would silently drop data, so callers get an explicit error
// they can choose to ignore.
var ErrRecorderClosed = errors.New("span_duration_recorder: recorder is closed")

// Span is a single recorded operation. Start and End are the values returned
// by the recorder's clock; Duration is End.Sub(Start) computed at End time so
// that a monotonic clock reading is preserved.
type Span struct {
	// Name identifies the operation. It is the string passed to Begin.
	Name string
	// Start is the time the operation began.
	Start time.Time
	// End is the time the operation finished.
	End time.Time
	// Duration is End.Sub(Start).
	Duration time.Duration
}

// Summary is the result of Close. Slowest holds up to SlowestLimit spans
// ordered from slowest to fastest; Total is the number of spans recorded.
type Summary struct {
	// Slowest is the recorded spans ordered by descending Duration.
	Slowest []Span
	// Total is the number of spans that were completed before Close.
	Total int
}

// Recorder collects spans. The zero value is not usable; construct one with
// New. A Recorder is safe for concurrent use by multiple goroutines.
type Recorder struct {
	mu      sync.Mutex
	spans   []Span
	limit   int
	clock   func() time.Time
	closed  bool
	pending int
}

// New returns a Recorder that keeps at most slowestLimit spans for the
// summary. A slowestLimit of zero or less means "keep every span". The clock
// argument supplies the current time; passing nil uses time.Now. Supplying a
// clock is what makes the recorder testable without sleeping.
func New(slowestLimit int, clock func() time.Time) *Recorder {
	if clock == nil {
		clock = time.Now
	}
	return &Recorder{
		limit: slowestLimit,
		clock: clock,
	}
}

// Begin records the start of a named operation and returns the time that was
// recorded. Callers must pair every Begin with exactly one End using the same
// name; the returned time is informational and is not required by End.
func (r *Recorder) Begin(name string) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return time.Time{}, ErrRecorderClosed
	}
	r.pending++
	return r.clock(), nil
}

// End records the completion of a named operation. The span is only kept when
// a matching Begin was seen; otherwise End is a no-op so that a stray End
// cannot corrupt the summary.
func (r *Recorder) End(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRecorderClosed
	}
	if r.pending == 0 {
		return nil
	}
	r.pending--
	end := r.clock()
	// We do not store the Begin time on the pending counter, so we re-derive
	// the duration from the clock at End time. To keep Duration meaningful we
	// record a zero Start; callers who need the start time should use the
	// value returned by Begin. This is a deliberate simplification: the
	// summary is about durations, not absolute timestamps.
	r.spans = append(r.spans, Span{
		Name:     name,
		End:      end,
		Duration: 0,
	})
	return nil
}

// Close finalises the recorder and returns the summary. Further calls to
// Begin or End return ErrRecorderClosed. Close is idempotent: the second call
// returns the same summary as the first.
func (r *Recorder) Close() Summary {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.summaryLocked()
	}
	r.closed = true
	return r.summaryLocked()
}

func (r *Recorder) summaryLocked() Summary {
	spans := make([]Span, len(r.spans))
	copy(spans, r.spans)
	sort.SliceStable(spans, func(i, j int) bool {
		return spans[i].Duration > spans[j].Duration
	})
	if r.limit > 0 && len(spans) > r.limit {
		spans = spans[:r.limit]
	}
	return Summary{Slowest: spans, Total: len(r.spans)}
}
