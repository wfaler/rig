# herdr Integration — Verification Checklist

This file verifies that `rig up` wired the container into [herdr](https://herdr.dev)
correctly. It is meant to be run **in the next session**, i.e. after:

1. Rebuilding the `rig` binary from this branch and putting it on the host `PATH`
   (`make build && cp rig <somewhere-on-PATH>` — the host binary carries the
   re-exec / socket-mount / env-injection logic).
2. Re-entering from a **herdr pane** so `HERDR_ENV=1` is set on the host:
   `rig rebuild` (the `.rig.yml` now has a `herdr:` block, so the config hash
   changed and the image rebuilds with the herdr CLI + skill), then `rig up`.

> The current (pre-change) container has **none** of the things below — that is
> the expected "before" state. Everything below should be present "after".

---

## 1. herdr environment variables are injected

```bash
env | grep HERDR | sort
```

**Expect:**
- `HERDR_ENV=1`
- `HERDR_SOCKET_PATH=/run/herdr/herdr.sock`  ← remapped in-container path
- `HERDR_AGENT=claude`                        ← from `.rig.yml` `herdr.agent`
- `HERDR_PANE_ID=…`, `HERDR_WORKSPACE_ID=…`, `HERDR_TAB_ID=…` (whatever herdr set)

## 2. herdr CLI is installed and on PATH

```bash
which herdr           # -> /usr/local/bin/herdr
herdr --version
herdr --help | head
```

## 3. The control socket is mounted and reachable

> **macOS note:** unix sockets cannot cross the Docker VM boundary (the
> bind-mounted file appears but `connect()` is refused — verified on OrbStack).
> On macOS rig instead bridges the socket over TCP: the host `rig` process
> listens on a deterministic loopback port (`RIG_HERDR_PROXY_PORT` in the
> container env) and the container entrypoint runs socat to re-expose it at
> `/run/herdr/herdr.sock`. On Linux the direct bind mount is used. Either way
> the checks below are identical.

```bash
ls -la /run/herdr/herdr.sock      # a socket (srwx...), not a directory
```

Then confirm the CLI can actually talk to herdr over it (this exercises the
mounted socket + `HERDR_SOCKET_PATH`):

```bash
herdr pane current   # should return pane JSON, not a connection error
                     # (bare `herdr pane` only prints usage and never touches the socket)
```

A raw protocol ping (herdr speaks newline-delimited JSON) if `nc` is available:

```bash
printf '{"id":"1","method":"ping","params":{}}\n' | nc -U /run/herdr/herdr.sock
# expect: {"id":"1","result":{"type":"pong"}}
```

## 4. The herdr agent skill is installed for Claude Code

```bash
cat ~/.claude/skills/herdr/SKILL.md | head -20
```

**Expect:** the skill file exists and mentions checking `HERDR_ENV=1` and using
the `herdr` CLI. Inside Claude Code, `/` should list a `herdr` skill.

## 5. Spawning sandboxed sessions from inside the container

The container exposes the host path of `/workspace` and a `rig-sandbox` skill
telling agents to launch new herdr panes through `rig up` (so spawned work
stays in the Docker sandbox rather than a bare host shell):

```bash
echo "$RIG_HOST_WORKDIR"                              # host path of /workspace
head -4 ~/.claude/skills/rig-sandbox/SKILL.md         # skill exists
```

End-to-end (from inside the container):

```bash
herdr pane split --direction right --cwd "$RIG_HOST_WORKDIR" --no-focus   # note pane_id
herdr pane run <pane_id> "rig up"
herdr pane wait-output <pane_id> --match "/workspace" --timeout 120000
herdr pane run <pane_id> "echo sandboxed: $(hostname)"    # runs in the container
herdr pane close <pane_id>                                # cleanup
```

## 6. Host-side detection (check in the herdr UI, not from the container)

From inside the container we can't see the host process tree. In herdr's own
pane/agent list on the host, the pane running `rig up` should be detected as the
**claude** agent (state idle/working/blocked), proving the re-exec exposed
`HERDR_AGENT=claude` to herdr's process detection.

---

## If something fails

| Symptom | Likely cause |
|---|---|
| No `HERDR_*` vars in the container | Not launched from a herdr pane, or host `rig` binary is the old one (no env injection) |
| `HERDR_AGENT` missing but others present | exec-env injection path issue in `internal/docker/attach.go` |
| Socket path exists but CLI can't connect | macOS: TCP bridge down — host `rig up` process not running, or socat failed (`/tmp/herdr-proxy.log` in the container). Linux: permissions (entrypoint `chmod`), or host↔container uid mismatch on the socket |
| `herdr` not on PATH / no skill | image wasn't rebuilt (`rig rebuild`), or `herdr.enabled` is false |
| Pane not detected as claude in herdr UI | the open item: herdr may not read `HERDR_AGENT` from the foreground `/proc/environ`. Fallback: launch with `HERDR_AGENT=claude rig up` |

Rendered nicely at `http://localhost:3031/docs/herdr-verification.md` (markdown server).
