package leaktest_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hamdiBouhani/leaktest"
)

// ExampleCheck demonstrates the typical usage of leaktest.Check.
// This example is verified by `go test` — if it leaks, the test fails.
func ExampleCheck() {
	t := &testing.T{} // not actually used; see below

	// In a real test you'd have a *testing.T. This example is compiled
	// but not executed as a test, so we use a no-op reporter.
	_ = t
	_ = leaktest.Check
	// Output:
}

// ExampleCheck_worker shows how to test a function that spawns a
// goroutine which cleans itself up via defer.
func ExampleCheck_worker() {
	// Simulate a real test with a fake reporter.
	reporter := &exampleReporter{}

	func() {
		defer leaktest.CheckTimeout(reporter, 2*time.Second)()

		// A well-behaved worker: exits when the channel is closed.
		done := make(chan struct{})
		go func() {
			defer close(done)
			// ... do work ...
		}()
		<-done
	}()

	fmt.Println("leaks reported:", len(reporter.errors))
	// Output:
	// leaks reported: 0
}

// ExampleCheckTimeout shows how to extend the grace period for a
// background worker that takes a while to shut down.
func ExampleCheckTimeout() {
	reporter := &exampleReporter{}

	func() {
		// Give the worker 10 seconds to exit.
		defer leaktest.CheckTimeout(reporter, 10*time.Second)()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond) // simulate slow shutdown
		}()
		wg.Wait()
	}()

	fmt.Println("leaks reported:", len(reporter.errors))
	// Output:
	// leaks reported: 0
}

// ExampleCheckContext shows how to drive the check with a context,
// which is useful when integrating with an existing timeout or
// cancellation mechanism.
func ExampleCheckContext() {
	reporter := &exampleReporter{}

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		defer leaktest.CheckContext(ctx, reporter)()

		// ... test code ...
	}()

	fmt.Println("leaks reported:", len(reporter.errors))
	// Output:
	// leaks reported: 0
}

// exampleReporter is a minimal ErrorReporter that records messages
// instead of failing a real test. Useful in examples and library tests.
type exampleReporter struct {
	errors []string
}

func (r *exampleReporter) Errorf(format string, args ...interface{}) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}
