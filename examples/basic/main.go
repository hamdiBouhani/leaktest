// Command basic demonstrates leaktest catching a real goroutine leak.
//
// Run with:
//
//	go run ./examples/basic
//
// The program intentionally leaks a goroutine to show what a leak report
// looks like. In real code you'd fix the leak rather than print it.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/hamdiBouhani/leaktest"
)

// consoleReporter prints errors to stderr, mimicking testing.T.
type consoleReporter struct{}

func (consoleReporter) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func main() {
	fmt.Println("=== Example 1: clean test ===")
	runClean()
	fmt.Println()

	fmt.Println("=== Example 2: leaked goroutine ===")
	runLeaky()
}

// runClean demonstrates a test that properly cleans up after itself.
func runClean() {
	reporter := consoleReporter{}

	func() {
		defer leaktest.CheckTimeout(reporter, 1*time.Second)()

		done := make(chan struct{})
		go func() {
			defer close(done)
			time.Sleep(50 * time.Millisecond)
		}()
		<-done
	}()

	fmt.Println("no leaks detected ✓")
}

// runLeaky demonstrates a test that forgets to stop a goroutine.
func runLeaky() {
	reporter := consoleReporter{}

	func() {
		defer leaktest.CheckTimeout(reporter, 500*time.Millisecond)()

		// Oops — this goroutine blocks forever.
		block := make(chan struct{})
		go func() {
			<-block
		}()
		// We never close(block).
	}()

	fmt.Println("(leak should have been reported above)")
}
