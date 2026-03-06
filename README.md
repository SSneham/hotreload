# hotreload

`hotreload` is a Go CLI that rebuilds and restarts your server when files change.

## Features

- Recursive file watching with `fsnotify`
- Ignore rules for common generated/temp paths
- 500ms debounce for change events
- Build command execution with live output
- Server process start/stop with child-process cleanup
- Crash-loop protection (3s restart delay if process exits within 1s)

## Project structure

- `cmd/hotreload`: CLI entrypoint
- `internal/watcher`: recursive watcher
- `internal/debounce`: event debounce utility
- `internal/builder`: build command runner
- `internal/process`: server process manager
- `testserver`: demo HTTP server for hotreload

## Requirements

- Go 1.26+
- `make` (optional, for Makefile targets)

## Quick start

### Windows (PowerShell)

```powershell
go build -o hotreload.exe ./cmd/hotreload
.\hotreload.exe --root .\testserver --build "go build -o .\bin\server.exe .\testserver" --exec ".\bin\server.exe"
```

### macOS/Linux

```bash
go build -o ./bin/hotreload ./cmd/hotreload
./bin/hotreload --root ./testserver --build "go build -o ./bin/server ./testserver" --exec "./bin/server"
```

## Makefile usage

```bash
make build         # build hotreload binary
make build-server  # build demo server
make run           # run hotreload with demo server settings (unix-like shell)
make fmt           # run gofmt
make test          # run go test ./...
make clean         # remove ./bin
```
