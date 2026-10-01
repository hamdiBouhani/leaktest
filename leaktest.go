// Package leaktest provides tools to detect leaked goroutines in tests.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//		defer leaktest.Check(t)()
//		// ... test code that may spawn goroutines ...
//	}
//
// The deferred call snapshots all "interesting" goroutines at test start,
// then at test end verifies that none of them are still running (with a
// grace period for goroutines that exit asynchronously).
package leaktest

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TickerInterval is the interval at which Check* re-scans for leaked
// goroutines while waiting for them to exit.
var TickerInterval = 50 * time.Millisecond

// DefaultTimeout is the grace period used by Check before declaring a leak.
const DefaultTimeout = 5 * time.Second

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// goroutine represents a single goroutine captured from runtime.Stack.
type goroutine struct {
	id    uint64
	stack string
}

// goroutineByID sorts goroutines by ID for deterministic output.
type goroutineByID []*goroutine

func (g goroutineByID) Len() int           { return len(g) }
func (g goroutineByID) Less(i, j int) bool { return g[i].id < g[j].id }
func (g goroutineByID) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }

// ErrorReporter is the minimal subset of testing.TB that leaktest needs.
// Using an interface (rather than *testing.T) keeps the package decoupled
// from the testing framework and makes it easy to unit test.
type ErrorReporter interface {
	Errorf(format string, args ...interface{})
}

// ---------------------------------------------------------------------------
// Stack parsing and filtering
// ---------------------------------------------------------------------------

// ignoreSubstrings lists stack fragments that should never be considered
// leaks. These are runtime internals, testing framework goroutines, and
// infrastructure goroutines that legitimately outlive a single test.
var ignoreSubstrings = []string{
	// Testing framework
	"testing.RunTests",
	"testing.Main(",
	"testing.(*T).Run(",

	// Runtime internals
	"runtime.goexit",
	"runtime.MHeap_Scavenger",
	"runtime_mcall",
	"created by runtime.gc",

	// Signal handling
	"signal.signal_recv",
	"sigterm.handler",

	// HTTP keep-alive loops (from net/http)
	").readLoop(",
	").writeLoop(",

	// The leaktest machinery itself
	"interestingGoroutines",
	"interestingGoroutine",

	// Goroutines executing C code
	"goroutine in C code",
}

// interestingGoroutine parses a single goroutine stack. It returns:
//   - (gr, nil)    if the goroutine is interesting (should be tracked)
//   - (nil, nil)   if the goroutine should be ignored
//   - (nil, err)   if the stack could not be parsed
func interestingGoroutine(g string) (*goroutine, error) {
	sl := strings.SplitN(g, "\n", 2)
	if len(sl) != 2 {
		return nil, fmt.Errorf("error parsing stack: %q", g)
	}

	stack := strings.TrimSpace(sl[1])
	if stack == "" {
		return nil, nil
	}

	for _, s := range ignoreSubstrings {
		if strings.Contains(stack, s) {
			return nil, nil
		}
	}

	// Header looks like: "goroutine 42 [running]:"
	h := strings.SplitN(sl[0], " ", 3)
	if len(h) < 3 {
		return nil, fmt.Errorf("error parsing stack header: %q", sl[0])
	}
	id, err := strconv.ParseUint(h[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing goroutine id: %w", err)
	}

	return &goroutine{id: id, stack: strings.TrimSpace(g)}, nil
}

// interestingGoroutines returns all goroutines worth tracking, sorted by ID.
// "capture every goroutine currently running in the process,
// filter out the uninteresting ones, and return the rest sorted by ID."
// //    runtime.Stack(buf, true)
// //             │
// //             ▼
// // ┌───────────────────────────┐
// // │ buf = "goroutine 1 ...\n  │
// // │        \n                 │
// // │        goroutine 2 ...\n  │
// // │        \n                 │
// // │        goroutine 3 ..."   │
// // └───────────────────────────┘
// //             │ strings.Split(_, "\n\n")
// //             ▼
// // ┌───────────────────────────┐
// // │ ["goroutine 1 ...",       │
// // │  "goroutine 2 ...",       │
// // │  "goroutine 3 ..."]       │
// // └───────────────────────────┘
// //             │ for each g: interestingGoroutine(g)
// //             ▼
// // ┌───────────────────────────┐
// // │ g1: interesting → keep    │
// // │ g2: ignore rule → drop    │
// // │ g3: interesting → keep    │
// // └───────────────────────────┘
// //             │ append
// //             ▼
// // ┌───────────────────────────┐
// // │ [*goroutine(1), *g(3)]    │
// // └───────────────────────────┘
// //             │ sort.Sort(goroutineByID(...))
// //             ▼
// // ┌───────────────────────────┐
// // │ [*goroutine(1), *g(3)]    │  ← ordered by id
// // └───────────────────────────┘
func interestingGoroutines(t ErrorReporter) []*goroutine {
	// 2<<20 is a bit-shift expression:
	//    1 << 20 = 2²⁰ = 1,048,576 = 1 MB
	//    2 << 20 = 2 × 2²⁰ = 2,097,152 = 2 MB
	buf := make([]byte, 2<<20)           // Allocates a 2 MB byte slice.
	buf = buf[:runtime.Stack(buf, true)] // Dump all goroutine stacks

	var gs []*goroutine
	// Split the stack dump into individual goroutine stacks and parse them.
	for _, g := range strings.Split(string(buf), "\n\n") {
		gr, err := interestingGoroutine(g)
		if err != nil {
			t.Errorf("leaktest: %s", err)
			continue
		}
		if gr == nil {
			continue
		}
		gs = append(gs, gr)
	}
	sort.Sort(goroutineByID(gs))
	return gs
}

// leakedGoroutines compares the current set of interesting goroutines
// against the original snapshot. It returns the stacks of any new
// goroutines and true if no leaks were found.
func leakedGoroutines(orig map[uint64]bool, interesting []*goroutine) ([]string, bool) {
	var leaked []string
	for _, g := range interesting {
		if !orig[g.id] {
			leaked = append(leaked, g.stack)
		}
	}
	return leaked, len(leaked) == 0
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Check snapshots the currently running goroutines and returns a function
// to be deferred at the end of a test. The returned function waits up to
// DefaultTimeout for any newly-spawned goroutines to exit before reporting
// them as leaks.
//
// Typical usage:
//
//	func TestFoo(t *testing.T) {
//		defer leaktest.Check(t)()
//		...
//	}
func Check(t ErrorReporter) func() {
	return CheckTimeout(t, DefaultTimeout)
}

// CheckTimeout is like Check but with a configurable timeout.
func CheckTimeout(t ErrorReporter, dur time.Duration) func() {
	ctx, cancel := context.WithCancel(context.Background())
	fn := CheckContext(ctx, t)
	return func() {
		timer := time.AfterFunc(dur, cancel)
		defer timer.Stop()
		defer cancel()
		fn()
	}
}

// CheckContext is like Check but uses a context for cancellation and
// timeout control. The returned function blocks until either no leaks are
// detected or ctx is cancelled (e.g. by a deadline).
// // // Phase 1: Setup (runs immediately)
// //
// //     CheckContext(ctx, t)
// //          │
// //          ▼
// //    ┌─────────────────────────────────────┐
// //    │ orig := make(map[uint64]bool)       │
// //    └─────────────────────────────────────┘
// //          │
// //          ▼
// //    ┌─────────────────────────────────────┐
// //    │ interestingGoroutines(t)            │
// //    │   → snapshot all current goroutines │
// //    │   → filter out runtime/testing/http │
// //    │   → sort by ID                      │
// //    └─────────────────────────────────────┘
// //          │
// //          ▼
// //    ┌─────────────────────────────────────┐
// //    │ for _, g := range ... {             │
// //    │     orig[g.id] = true               │
// //    │ }                                   │
// //    │   → record IDs in a set             │
// //    └─────────────────────────────────────┘
// //          │
// //          ▼
// //    ┌─────────────────────────────────────┐
// //    │ return func() { ... }               │
// //    │   → capture orig in closure         │
// //    └─────────────────────────────────────┘
// //          │
// //          ▼
// //    The returned closure is what
// //    `defer` will invoke at test end.
// //
// // // Phase 2: Check (runs when defer fires)
// //             ┌───────────────────────────┐
// //             │   Returned closure runs   │
// //             └───────────────────────────┘
// //                         │
// //                         ▼
// //         ┌───────────────────────────────┐
// //         │  FAST PATH                    │
// //         │  leaked, ok = leakedGoroutines│
// //         │    (orig,                     │
// //         │     interestingGoroutines(t)) │
// //         └───────────────────────────────┘
// //                         │
// //               ┌─────────┴─────────┐
// //               │                   │
// //            ok=true             ok=false
// //         (no leaks)          (leaks present)
// //               │                   │
// //               ▼                   ▼
// //         ┌──────────┐    ┌──────────────────────┐
// //         │  RETURN  │    │  Start Ticker        │
// //         │  (clean) │    │  time.NewTicker(50ms)│
// //         └──────────┘    └──────────────────────┘
// //                                   │
// //                                   ▼
// //                         ┌──────────────────┐
// //                         │   POLL LOOP      │
// //                         │   for { select } │
// //                         └──────────────────┘
// //                                   │
// //            ┌──────────────────────┼──────────────────────┐
// //            │                      │                      │
// //       ticker.C                ticker.C               ctx.Done()
// //       (50ms later)            (100ms later)          (cancelled)
// //            │                      │                      │
// //            ▼                      ▼                      ▼
// //   ┌────────────────┐    ┌────────────────┐    ┌─────────────────┐
// //   │ re-scan        │    │ re-scan        │    │ REPORT          │
// //   │ leakedGoroutines│   │ leakedGoroutines│   │ t.Errorf(ctx)   │
// //   └────────────────┘    └────────────────┘    │ for each leak:  │
// //            │                      │           │   t.Errorf(...) │
// //      ┌─────┴─────┐         ┌─────┴─────┐     └─────────────────┘
// //      │           │         │           │             │
// //   ok=true     ok=false   ok=true    ok=false          ▼
// //      │           │         │           │          ┌──────────┐
// //      ▼           │         ▼           │          │  RETURN  │
// // ┌──────────┐     │    ┌──────────┐     │          │ (leaks!) │
// // │  RETURN  │     │    │  RETURN  │     │          └──────────┘
// // │ (clean)  │     │    │ (clean)  │     │
// // └──────────┘     │    └──────────┘     │
// //                  │                     │
// //                  └─────────┬───────────┘
// //                            │
// //                            ▼
// //                       (loop again)
func CheckContext(ctx context.Context, t ErrorReporter) func() {
	// Snapshot interesting goroutines at start.
	// orig is a set of IDs, not a list of stacks
	// IDs are monotonically increasing and never reused within a process
	// // // 	   orig = {
	// // //      1:    true,   // main goroutine
	// // //      18:   true,   // runtime scavenger
	// // //      23:   true,   // testing harness
	// // //      42:   true,   // some background worker
	// // //      ...
	// // //    }
	orig := make(map[uint64]bool)
	for _, g := range interestingGoroutines(t) {
		orig[g.id] = true
	}

	return func() {
		// Fast path: check once before spinning up a ticker.
		leaked, ok := leakedGoroutines(orig, interestingGoroutines(t))
		if ok {
			return
		}

		ticker := time.NewTicker(TickerInterval)
		defer ticker.Stop()

		// Poll until the leaks disappear or the context is cancelled.
		for {
			select {
			case <-ticker.C:
				leaked, ok = leakedGoroutines(orig, interestingGoroutines(t))
				if ok {
					return
				}
			case <-ctx.Done():
				t.Errorf("leaktest: %v", ctx.Err())
				// Fall through and report the last known leaks.
				for _, g := range leaked {
					t.Errorf("leaktest: leaked goroutine: %v", g)
				}
				return
			}
		}
	}
}
