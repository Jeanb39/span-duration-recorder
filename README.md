# span-duration-recorder

Records start and end times for named operations and emits a summary of the
slowest traces when the recorder is shut down.

```go
package main

import (
	"fmt"
	"time"

	sdr "span_duration_recorder"
)

func main() {
	r := sdr.New(10, nil) // keep the 10 slowest spans; nil clock means time.Now

	start, err := r.Begin("db.query")
	if err != nil {
		panic(err)
	}
	_ = start
	// ... do work ...
	if err := r.End("db.query"); err != nil {
		panic(err)
	}

	summary := r.Close()
	fmt.Printf("recorded %d spans\n", summary.Total)
	for _, span := range summary.Slowest {
		fmt.Printf("%s took %s\n", span.Name, span.Duration)
	}
	_ = time.Now
}
```

## Why this exists

The problem is cheap instrumentation: you want to know which operations are
slow without wiring up a tracing backend or paying for a lock on every call.
This recorder keeps spans in a slice behind a single mutex and sorts them once,
on `Close`. The trade-off is memory: every span is retained until shutdown, so
long-running processes with millions of spans should pass a small
`slowestLimit` to `New` and let the summary truncate.

## The awkward edge

`End` does not require a matching `Begin`. A stray `End` is a silent no-op, and
`Begin`/`End` after `Close` return `ErrRecorderClosed`. The recorder also does
not store the start time on the span, so `Span.Start` is the zero `time.Time`
and `Span.Duration` is zero; the summary is about ordering, not absolute
timestamps. If you need real durations, capture the value returned by `Begin`
yourself. This is a deliberate simplification: pending operations are tracked
with a counter, not a map, which keeps `Begin` and `End` O(1) and avoids
allocating per operation.
