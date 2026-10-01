# Contributing

## Prerequisites

- Go 1.18+
- [Task](https://taskfile.dev/) (recommended) or `make`

## Common Tasks

```bash
task test        # run tests
task test:race   # run with race detector
task check       # fmt-check + vet + test (what CI runs)
task examples    # run the runnable examples
task cover       # generate coverage.html