# crawler

This package contains a basic crawler built on top of Bokchoy.

## Usage

### Producer

```console
go run main.go -run producer -url https://golang.org/
```

By default the depth is `1`, you can override it with `-depth` flag.

### Worker

```console
go run main.go -run worker
```

By default the concurrency is `1`, you can override it with `-concurrency` flag.
