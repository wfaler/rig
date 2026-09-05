---
name: rig-sandbox
description: "Spawn herdr panes whose work runs inside rig sandboxes (Docker containers) instead of on the host. Use whenever this session runs inside a rig container (RIG_HOST_WORKDIR is set) and you are about to create herdr panes, tabs, or workspaces for parallel or delegated work. Requires HERDR_ENV=1."
---

# Sandboxed herdr sessions with rig

This session runs inside a **rig container**: the directory you see as
`/workspace` is the host directory `$RIG_HOST_WORKDIR` mounted into a Docker
sandbox. New herdr panes, however, are spawned by herdr **on the host** — a
bare `herdr pane split` hands you an unsandboxed host shell. To keep spawned
work sandboxed, every new pane must launch `rig up` before doing anything else.

## Preconditions

```bash
test "${HERDR_ENV:-}" = 1 && test -n "${RIG_HOST_WORKDIR:-}"
```

If this fails, you are not in a herdr-managed rig container; stop and say so.

## Spawning a sandboxed pane

1. Split using the **host** path — never `/workspace`, which does not exist on
   the host:

   ```bash
   herdr pane split --direction right --cwd "$RIG_HOST_WORKDIR" --no-focus
   ```

   Note the new `pane_id` in the JSON response.

2. Start the sandbox in the new pane:

   ```bash
   herdr pane run <pane_id> "rig up"
   ```

3. Wait until the container shell is up before sending work:

   ```bash
   herdr pane wait-output <pane_id> --match "/workspace" --timeout 120000
   ```

   The first `rig up` for a changed config builds an image and can take
   minutes; subsequent ones attach in seconds.

4. Send work with `herdr pane run <pane_id> "<command>"` or
   `herdr pane send-text`. Commands now execute inside the rig container —
   for the same `--cwd`, the *same* container as this session (shared
   filesystem and processes, separate shell).

To sandbox a **different** project, pass that project's host directory as
`--cwd`; `rig up` there starts or attaches that project's own container.

## Rules

- Never run work-related commands in a spawned pane before `rig up` has
  attached — until then the pane is a host shell, outside the sandbox.
- If a spawned pane's `rig up` fails, close the pane (`herdr pane close`)
  rather than falling back to running work on the host.
