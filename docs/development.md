# Development

The root README intentionally avoids local build and run instructions. This page contains the development workflow for contributors.

## Useful Make targets

```sh
make download
make fmt
make vet
make test
make cover
make build
make run
```

Run a local example with webhook and email settings:

```sh
make run-test
```

Send local requests:

```sh
make check-in
make check-in-details
make status
make status-details
```

## Build locally

```sh
go build -o overdue .
```

## Run locally

```sh
./overdue \
  --expected-every=5m \
  --alerting-delay=30s
```

Or with environment variables:

```sh
export OVERDUE__EXPECTED_EVERY=5m
export OVERDUE__ALERTING_DELAY=30s
./overdue
```

## Run tests

```sh
go test ./...
```

The project Makefile may add formatting, vetting, coverage, or timeout settings around the raw Go commands.
