package span_duration_recorder

import (
	"errors"
	"testing"
	"time"
)

// fakeClock returns a clock function that advances by step on every call.
// Tests use it instead of sleeping so the suite is deterministic.
func fakeClock(start time.Time, step time.Duration) func() time.Time {
	cur := start
	return func() time.Time {
		t := cur
		cur = cur.Add(step)
		return t
	}
}

func TestBeginEndRecordsSpan(t *testing.T) {
	r := New(0, fakeClock(time.Unix(0, 0), time.Millisecond))
	if _, err := r.Begin("op"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := r.End("op"); err != nil {
		t.Fatalf("End: %v", err)
	}
	s := r.Close()
	if s.Total != 1 {
		t.Fatalf("Total = %d, want 1", s.Total)
	}
	if len(s.Slowest) != 1 {
		t.Fatalf("Slowest len = %d, want 1", len(s.Slowest))
	}
	if s.Slowest[0].Name != "op" {
		t.Fatalf("Name = %q, want %q", s.Slowest[0].Name, "op")
	}
}

func TestEndWithoutBeginIsNoop(t *testing.T) {
	r := New(0, fakeClock(time.Unix(0, 0), time.Millisecond))
	if err := r.End("ghost"); err != nil {
		t.Fatalf("End: %v", err)
	}
	s := r.Close()
	if s.Total != 0 {
		t.Fatalf("Total = %d, want 0", s.Total)
	}
}

func TestBeginAfterCloseReturnsError(t *testing.T) {
	r := New(0, fakeClock(time.Unix(0, 0), time.Millisecond))
	r.Close()
	if _, err := r.Begin("late"); !errors.Is(err, ErrRecorderClosed) {
		t.Fatalf("Begin err = %v, want ErrRecorderClosed", err)
	}
}

func TestEndAfterCloseReturnsError(t *testing.T) {
	r := New(0, fakeClock(time.Unix(0, 0), time.Millisecond))
	r.Close()
	if err := r.End("late"); !errors.Is(err, ErrRecorderClosed) {
		t.Fatalf("End err = %v, want ErrRecorderClosed", err)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	r := New(0, fakeClock(time.Unix(0, 0), time.Millisecond))
	r.Begin("a")
	r.End("a")
	first := r.Close()
	second := r.Close()
	if first.Total != second.Total {
		t.Fatalf("Total differs: %d vs %d", first.Total, second.Total)
	}
	if len(first.Slowest) != len(second.Slowest) {
		t.Fatalf("Slowest len differs: %d vs %d", len(first.Slowest), len(second.Slowest))
	}
}

func TestSlowestLimitTruncates(t *testing.T) {
	r := New(2, fakeClock(time.Unix(0, 0), time.Millisecond))
	for i := 0; i < 5; i++ {
		r.Begin("op")
		r.End("op")
	}
	s := r.Close()
	if s.Total != 5 {
		t.Fatalf("Total = %d, want 5", s.Total)
	}
	if len(s.Slowest) != 2 {
		t.Fatalf("Slowest len = %d, want 2", len(s.Slowest))
	}
}

func TestSlowestLimitZeroKeepsAll(t *testing.T) {
	r := New(0, fakeClock(time.Unix(0, 0), time.Millisecond))
	for i := 0; i < 3; i++ {
		r.Begin("op")
		r.End("op")
	}
	s := r.Close()
	if len(s.Slowest) != 3 {
		t.Fatalf("Slowest len = %d, want 3", len(s.Slowest))
	}
}

func TestNilClockUsesTimeNow(t *testing.T) {
	r := New(0, nil)
	if _, err := r.Begin("op"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := r.End("op"); err != nil {
		t.Fatalf("End: %v", err)
	}
	s := r.Close()
	if s.Total != 1 {
		t.Fatalf("Total = %d, want 1", s.Total)
	}
}

func TestSummarySlowestIsCopy(t *testing.T) {
	r := New(0, fakeClock(time.Unix(0, 0), time.Millisecond))
	r.Begin("a")
	r.End("a")
	s := r.Close()
	if len(s.Slowest) == 0 {
		t.Fatal("expected at least one span")
	}
	s.Slowest[0].Name = "mutated"
	again := r.Close()
	if again.Slowest[0].Name == "mutated" {
		t.Fatal("summary shares backing array with recorder")
	}
}
