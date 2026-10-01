// Command http-server demonstrates leaktest catching an HTTP server
// that was not shut down properly.
//
// Run with:
//
//	go run ./examples/http-server
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hamdiBouhani/leaktest"
)

type consoleReporter struct{}

func (consoleReporter) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func main() {
	fmt.Println("=== Example 1: server shut down cleanly ===")
	runCleanServer()
	fmt.Println()

	fmt.Println("=== Example 2: server not shut down ===")
	runLeakyServer()
}

func runCleanServer() {
	reporter := consoleReporter{}

	func() {
		defer leaktest.CheckTimeout(reporter, 2*time.Second)()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		srv := startServer(ctx)
		defer srv.Shutdown(context.Background())

		// ... test the server ...
		time.Sleep(50 * time.Millisecond)
	}()

	fmt.Println("no leaks detected ✓")
}

func runLeakyServer() {
	reporter := consoleReporter{}

	func() {
		defer leaktest.CheckTimeout(reporter, 1*time.Second)()

		srv := startServer(context.Background())
		_ = srv
		// Oops — we never call srv.Shutdown.
	}()

	fmt.Println("(leak should have been reported above)")
}

func startServer(ctx context.Context) *http.Server {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "hello")
		}),
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	go srv.Serve(ln)
	return srv
}
