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