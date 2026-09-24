# CS425 MP2 — Distributed Group Membership

Gossip-style heartbeating, with an optional suspicion mode (Gossip+S).  
Commands must be typed in the **already running server window**, not in a new PowerShell/bash prompt.

## Requirements

- Go 1.21+
- From this directory (`mp2/`)

## Compile

```powershell
cd mp2
go build ./...
```

Run without installing a binary:

```powershell
go run ./cmd/server [flags]
```

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-id` | `1` | Local machine id (used in logs and NodeID). Must be unique per process. |
| `-host` | `localhost` | Address this node listens on |
| `-port` | `8080` | Port this node listens on |
| `-introducer` | `false` | This node is the introducer |
| `-intro-host` | `localhost` | Introducer host (non-introducer only) |
| `-intro-port` | `8080` | Introducer port (non-introducer only) |
| `-suspect` | `false` | Start in Gossip+S instead of pure gossip |

NodeID format: `id:host:port:timestamp`  
Logs: `mp2/logs/machine.<id>.log` (also printed on the terminal)

## How to run (local)

Open one terminal per node. Always start the introducer first.

**Terminal 1 — introducer**

```powershell
cd mp2
go run ./cmd/server -id 1 -host localhost -port 8080 -introducer
```

**Terminal 2 — member**

```powershell
cd mp2
go run ./cmd/server -id 2 -host localhost -port 8081 -intro-host localhost -intro-port 8080
```

**Terminal 3 — another member**

```powershell
cd mp2
go run ./cmd/server -id 3 -host localhost -port 8082 -intro-host localhost -intro-port 8080
```

Start with suspicion enabled:

```powershell
go run ./cmd/server -id 1 -host localhost -port 8080 -introducer -suspect
```

Wait until you see `server listening` / `Successfully joined`, then type CLI commands in that same window.

On course VMs, replace `localhost` with the VM hostname/IP, and give each VM its own `-id` and `-port`.

## CLI commands

Type these after the process is running.

### `list_mem`

Print the local membership list (NodeID, heartbeat, incarnation, status).

```text
list_mem
```

### `list_self`

Print this process’s NodeID.

```text
list_self
```

### `join`

Non-introducer nodes already join at startup. Use this to join again (for example after a failed first attempt):

```text
join
join localhost 8080
```

Default introducer is `-intro-host` / `-intro-port` from startup.

### `leave`

Voluntarily leave, gossip the leave, then exit. This is **not** a crash.

```text
leave
```

### `display_suspects`

List every node this process has ever suspected, with the local time of the suspicion.

```text
display_suspects
```

### `switch`

Change protocol on **this node only**. Run it on every node if you want the whole group in the same mode.

```text
switch suspect
switch nosuspect
```

### `display_protocol`

Show whether this node is in pure gossip or Gossip+S.

```text
display_protocol
```

### `set_drop_rate`

Drop a fraction of **incoming** gossip messages on this receiver (`0` = no drops, `0.1` = 10%).

```text
set_drop_rate 0
set_drop_rate 0.1
```

## Crash vs leave (demo)

- **Leave:** type `leave` in that node’s window.
- **Crash / fail:** force-kill the process. Do not type `leave`, and do not shut down the VM.

Windows (PowerShell), after finding the PID:

```powershell
Stop-Process -Id <pid> -Force
```

Linux / course VM:

```bash
kill -9 <pid>
```

Other nodes should mark the dead node `Failed` (or `Suspect` then `Failed` if suspicion is on), then remove it after cleanup.

## Grep logs with MP1

Logs are at `mp2/logs/machine.<id>.log`. From `mp1/`, point the MP1 client at those files to search for join / leave / fail / suspect / drop lines.

## Tests

There are no automated unit tests in this folder yet. Manual demo checklist:

1. Start introducer, then 1–2 members; `list_mem` on each sees everyone.
2. `leave` on one member; others drop it within a few seconds.
3. Force-kill one member; others detect failure (and suspicion if enabled).
4. Restart the killed member with a new process (new timestamp); it rejoins as a new NodeID.
5. `switch suspect` / `switch nosuspect` on each node; `display_protocol` matches.
6. `set_drop_rate 0.1` and confirm drop lines in the log.
