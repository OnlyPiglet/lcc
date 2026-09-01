# Go implementation

This repository contains a dependency-free Go implementation of `every`.
It uses native per-user schedulers on all three desktop platforms:

- macOS: `launchd` agents under `~/Library/LaunchAgents`
- Linux: `systemd --user` service/timer pairs under `~/.config/systemd/user`
- Windows: Task Scheduler tasks in the `\\every\\` namespace

## Build

Go 1.21 or newer is required.

```sh
./build.sh
```

On Windows PowerShell:

```powershell
.\build.ps1
```

Install the resulting binary somewhere stable on `PATH` (for example
`~/bin/every` on Unix or `%LOCALAPPDATA%\Programs\every\every.exe` on
Windows). Scheduled configurations store the executable path, so replacing the
binary in the same location upgrades future runs without re-registering tasks.

## Use

```sh
every 15m -- 'sync-notes --quiet'
every day 9am,6pm --name reports -- './bin/report.sh'
every weekdays 09:30 --timeout 30m -- make standup

every list --json
every run reports
every log reports -n 40
every pause reports
every resume reports
every rm reports
every doctor
```

Commands are executed by the user's shell from the directory where the task
was created. Output is bounded in memory, written to a per-task log, and recent
runs are kept in a JSONL ledger. A non-zero command exit is preserved; a timed
out command exits with `124`.

`EVERY_HOME` overrides the data directory. Unix installations also honor
`XDG_DATA_HOME` and `XDG_CONFIG_HOME`; Windows uses `LOCALAPPDATA` and
`APPDATA` by default. Native Windows interval schedules must be at least one
minute because that is the smallest reliable Task Scheduler repetition.
Windows tasks use the current interactive user, so they run while that user is
logged in. Linux timers require a user systemd session (enable lingering with
`loginctl enable-linger "$USER"` when they must fire after logout).

## Verify

```sh
go test ./...
go vet ./...
GOOS=linux GOARCH=amd64 go build ./cmd/every
GOOS=darwin GOARCH=arm64 go build ./cmd/every
GOOS=windows GOARCH=amd64 go build ./cmd/every
```

On Windows, `windows_native_test.go` exercises `cmd.exe`, PowerShell,
Task Scheduler XML, timeout handling, Windows path resolution, BOM parsing,
and atomic file replacement.
