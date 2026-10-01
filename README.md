# leaktest

[![Go Reference](https://pkg.go.dev/badge/github.com/hamdiBouhani/leaktest.svg)](https://pkg.go.dev/github.com/hamdiBouhani/leaktest)
[![Go Report Card](https://goreportcard.com/badge/github.com/hamdiBouhani/leaktest)](https://goreportcard.com/report/github.com/hamdiBouhani/leaktest)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE)

`leaktest` is a small Go library for detecting **leaked goroutines** in tests.

It snapshots the goroutines running at the start of a test, then at the end
verifies that none of them are still running (with a short grace period for
goroutines that exit asynchronously). If a goroutine has leaked, the test
fails with the full stack trace of the offending goroutine.

---

## Why?

Goroutine leaks are one of the most common — and most insidious — bugs in
concurrent Go code. A goroutine that never exits:

- Holds references to memory that can never be reclaimed
- May hold locks, file descriptors, or network connections indefinitely
- Silently degrades performance over the life of a long-running process

The Go runtime has no built-in way to detect them. `go test -race` catches
data races, not leaks. `pprof` shows goroutines but doesn't tell you which
ones *shouldn't* be there.

`leaktest` fills that gap by making leak detection a one-line addition to
any test.

---

## Installation

```bash
go get github.com/hamdiBouhani/leaktest
```
Requires Go 1.18 or later (uses context and error wrapping).

## Usage
Add a single deferred call at the top of any test that spawns goroutines:

```go
package mypkg

import (
    "testing"

    "github.com/hamdiBouhani/leaktest"
)

func TestSomething(t *testing.T) {
    defer leaktest.Check(t)()

    // ... test code that may spawn goroutines ...
}
```
Note the double call. leaktest.Check(t) takes the initial snapshot
immediately and returns a function; the trailing () invokes that function
at defer time. Forgetting the second () is a common mistake and will cause
the check to never run.

If the test leaks a goroutine, you'll see something like:

```text 
--- FAIL: TestSomething (5.02s)
    leaktest: context deadline exceeded
    leaktest: leaked goroutine: goroutine 42 [chan receive]:
        main.worker()
            /tmp/main.go:20 +0x40
        created by main.TestSomething
            /tmp/main_test.go:15 +0x80
```

## Put the defer first
Place the defer at the very top of the test body, before any goroutines
are spawned. Otherwise the initial snapshot will include the very goroutines
you're trying to track, and leaks will go undetected.

```go
func TestGood(t *testing.T) {
    defer leaktest.Check(t)()
    go worker()  // spawned after snapshot — will be caught if it leaks
}

func TestBad(t *testing.T) {
    go worker()  // spawned before snapshot — will NOT be caught
    defer leaktest.Check(t)()
}
```