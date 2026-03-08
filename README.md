# hotreload: A Go-Based Live Rebuild and Restart System

A command-line development tool that watches project files and automatically rebuilds and restarts a server when code changes.

## What problem this tool solves

When I am iterating on backend code, the repetitive loop of "save file -> stop server -> rebuild -> rerun" breaks focus.  
The motivation behind `hotreload` was to collapse that loop into one continuous workflow so development feels closer to instant feedback.

Most editors can trigger multiple file events for a single save, and many projects have noisy folders (`.git`, `node_modules`, build outputs, temporary files).  
So the real challenge is not just watching files; it is watching the right files, reacting once, and restarting the server safely every time.

## What the system actually does

In simple terms, `hotreload` sits between your file saves and your running server.

1. It watches your project recursively for changes.
2. It waits briefly (debounce window) to avoid duplicate rebuilds.
3. It runs your build command.
4. If build succeeds, it terminates the old server process tree.
5. It starts the new server process.
6. It streams server logs directly to your terminal in real time.

You provide:
- `--root`: folder to watch
- `--build`: build command to run
- `--exec`: command to start the server

## Architecture / data flow

```text
File Save
   ↓
Filesystem Watcher (fsnotify)
   ↓
Event Debounce
   ↓
Build Command
   ↓
Stop Previous Server
   ↓
Start New Server
   ↓
Stream Logs to Terminal
```

## Breaking down the core problems solved

### Detecting file changes

The watcher is built on `github.com/fsnotify/fsnotify`.  
It watches the root directory and all subdirectories, and it adds new directories at runtime as they are created.  
It also handles deleted/renamed folders by removing their watches.

Noise control is built in:
- Ignored directories: `.git`, `node_modules`, `bin`, `build`, `tmp`
- Ignored file suffixes: `.swp`, `.tmp`, `.log`

### Preventing redundant rebuilds

File save operations often emit several low-level events.  
To avoid rebuilding multiple times for one logical change, the system applies a 500ms debounce stage.

The rebuild pipeline also handles in-flight changes:
- If a build is already running and a newer change arrives, the old build is canceled.
- Only the latest state is built and used for restart.

### Managing server processes

The process manager owns server lifecycle:
- Start server from a command string
- Stop server on rebuild/shutdown
- Kill full process tree (parent + children), not just parent
- Wait for process exit to prevent orphaned processes/zombies

It also includes crash-loop protection:
- If the server exits in under 1 second, restart is delayed by 3 seconds.

### Streaming logs

Server stdout/stderr are attached directly to terminal stdout/stderr.  
This means logs appear immediately as they are produced, rather than being buffered and dumped later.

## How the hot reload system works internally

At a high level:

1. CLI parses `--root`, `--build`, `--exec`.
2. Watcher starts recursive watch from `--root`.
3. Debouncer converts bursty events into a single rebuild trigger.
4. Rebuild coordinator runs build jobs with cancellation support.
5. On successful build:
   - stop previous server completely
   - start new server process
6. On build failure:
   - keep current server running
   - continue watching for next change

This separation into `watcher`, `debounce`, `builder`, and `process` keeps responsibilities clear and makes each part easier to reason about.

## Example run of the system

Example PowerShell session:

```powershell
.\hotreload.exe --root .\testserver --build "go build -o .\bin\server.exe .\testserver" --exec ".\bin\server.exe"
```

Typical output:

```text
time=... level=INFO msg="running build" command="go build -o .\\bin\\server.exe .\\testserver"
time=... level=INFO msg="build succeeded"
server started
time=... level=INFO msg="detected file changes"
time=... level=INFO msg="running build" command="go build -o .\\bin\\server.exe .\\testserver"
time=... level=INFO msg="build succeeded"
server started
```

## Project structure

- `cmd/hotreload/main.go`: CLI entrypoint and rebuild orchestration
- `internal/watcher`: recursive fsnotify watcher + ignore filters
- `internal/debounce`: cooldown logic to coalesce burst events
- `internal/builder`: build command execution (with cancellation/retry behavior)
- `internal/process`: server process lifecycle and process-tree termination
- `testserver`: simple HTTP server used to demonstrate hot reload behavior

## Quick start instructions

### Prerequisites

- Go 1.26+
- Optional: `make` for convenience targets

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

### Optional Makefile commands

```bash
make build
make build-server
make run
make fmt
make test
make clean
```

## Key design decisions

1. `fsnotify` as event source, custom orchestration for everything else  
Reason: reliable native file events without depending on external hot-reload frameworks.

2. Debounce before build execution  
Reason: editors generate noisy event bursts; a debounce window reduces wasted rebuilds.

3. Cancellable build pipeline  
Reason: if a newer change arrives, old work should be discarded so the system converges to latest state quickly.

4. Hard process-tree termination on restart  
Reason: gentle shutdown is not always enough; stubborn child processes must be cleaned up for stability.

5. Ignore filters for large/noisy directories  
Reason: improves signal quality and reduces unnecessary watcher/build load.

6. Real-time stdout/stderr streaming  
Reason: preserves normal server observability and shortens debugging feedback loops.

## What I learned from building this system

The biggest lesson was that hot reload is mostly a coordination problem, not a file-watch problem.  
Watching files is straightforward; making rebuild/restart behavior predictable under noisy, real-world conditions is the hard part.

Three practical takeaways:

- Event storms are normal. Debounce and cancellation are mandatory.
- Process management must be strict. If you do not kill process trees and wait for exit, orphans accumulate.
- Cross-platform behavior matters early, especially for executable rebuilding and process termination semantics on Windows.

## Demo video

Here is a video briefly demonstrating the project:

[Demo Video](https://www.loom.com/share/84884494b0fc4413a79174e44eddff4e)
