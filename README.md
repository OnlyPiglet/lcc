# every

`every` is a small local task scheduler with native launchd, systemd, and
Windows Task Scheduler backends. This repository contains the dependency-free
Go implementation.

## Build

Go 1.21 or newer is required.

On macOS or Linux:

```sh
./build.sh
go test ./...
```

On Windows PowerShell:

```powershell
.\build.ps1
go test ./...
```

The default output is `dist/every` on Unix and `dist/every.exe` on Windows.
The resulting executable should be installed at a stable path on `PATH` so
newly registered tasks continue to work after replacing the binary.

To cross-build a Windows binary from Unix:

```sh
GOOS=windows GOARCH=amd64 go build -trimpath -o dist/every.exe ./cmd/every
```

## Windows behavior

Windows tasks are registered under `\\every\\` and run as the current
interactive user. Task Scheduler intervals must be at least one minute.
Commands run through `cmd.exe` by default; set `EVERY_SHELL` to a PowerShell
executable when PowerShell syntax is required. Command output is captured in
the per-task data directory under `%LOCALAPPDATA%\every`.

## Commands

```text
every 15m -- "sync-notes --quiet"
every day 9am --name reports -- "./bin/report.cmd"
every list
every run reports
every pause reports
every resume reports
every rm reports
every doctor
```
