package leaktest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeReporter captures Errorf calls for inspection.
type fakeReporter struct {
	mu       sync.Mutex
	messages []string
}

func (r *fakeReporter) Errorf(format string, args ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, fmt.Sprintf(format, args...))
}

func (r *fakeReporter) contains(substr string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.messages {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

func TestNoLeak(t *testing.T) {
	defer Check(t)()
	// Do some work that spawns and joins a goroutine.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
	}()
	wg.Wait()
}

func TestLeakDetected(t *testing.T) {
	r := &fakeReporter{}
	done := CheckTimeout(r, 200*time.Millisecond)
	defer done()

	// Spawn a goroutine that never exits.
	block := make(chan struct{})
	go func() {
		<-block
	}()
	// Note: we never close(block), so the goroutine leaks.

	// We can't easily assert from inside the deferred function because
	// it's called after this test function returns. Instead, run the
	// deferred function manually here.
	done()
	if !r.contains("leaked goroutine") {
		t.Fatalf("expected leak to be reported, got %v", r.messages)
	}
}

func TestGoroutineExitsDuringGracePeriod(t *testing.T) {
	r := &fakeReporter{}
	done := CheckTimeout(r, 2*time.Second)
	defer done()

	// Spawn a goroutine that exits after a delay shorter than the timeout.
	go func() {
		time.Sleep(200 * time.Millisecond)
	}()

	done()
	if r.contains("leaked goroutine") {
		t.Fatalf("unexpected leak reported: %v", r.messages)
	}
}

func TestContextCancellation(t *testing.T) {
	r := &fakeReporter{}
	ctx, cancel := context.WithCancel(context.Background())
	done := CheckContext(ctx, r)
	defer done()

	// Leak a goroutine.
	block := make(chan struct{})
	defer close(block)
	go func() {
		<-block
	}()

	// Cancel the context immediately so the check fails fast.
	cancel()

	done()
	if !r.contains("context canceled") {
		t.Fatalf("expected context cancellation, got %v", r.messages)
	}
}

func TestInterestingGoroutineFiltering(t *testing.T) {
	// A stack containing a filtered substring should be ignored.
	ignored := []string{
		"goroutine 1 [running]:\ntesting.Main(...)",
		"goroutine 2 [running]:\nruntime.goexit()",
		"goroutine 3 [running]:\n...).readLoop(...)",
	}
	for _, s := range ignored {
		gr, err := interestingGoroutine(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gr != nil {
			t.Errorf("expected %q to be filtered, got %+v", s, gr)
		}
	}

	// A normal stack should be kept.
	kept := "goroutine 42 [running]:\nmain.main()\n\t/tmp/main.go:10"
	gr, err := interestingGoroutine(kept)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gr == nil || gr.id != 42 {
		t.Fatalf("expected goroutine 42 to be kept, got %+v", gr)
	}
}
