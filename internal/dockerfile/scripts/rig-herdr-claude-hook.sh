#!/bin/sh
# rig-herdr-claude-hook: keep herdr's status for this pane at "working" while
# Claude Code has background work in flight after its turn ends.
#
# herdr reads Claude Code's state from the screen. Once a turn ends, the
# prompt box returns and herdr shows the pane as idle, even though background
# shells, subagents, monitors or workflows are still running. Claude Code's
# Stop hook lists that work in `background_tasks`, so on Stop this script
# reports "working" to herdr, which takes precedence over screen detection.
# Every other hooked event releases the pane back to screen detection, which
# handles normal turns, permission prompts and interrupts.
#
# Usage:
#   rig-herdr-claude-hook.sh stop       Stop hook: report or release
#   rig-herdr-claude-hook.sh release    any other hook: release
#   rig-herdr-claude-hook.sh install    add the hooks to Claude's settings.json
#
# The hook commands always exit 0 and print nothing: herdr must never slow
# down or break Claude Code, and UserPromptSubmit hook output would be added
# to Claude's context.
set -u

SOURCE="rig:claude"
AGENT="claude"

# install merges this script's hook entries into Claude Code's settings.json,
# replacing any earlier entries for this script and keeping everything else.
install_hooks() {
    self="$(readlink -f "$0")"
    dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
    settings="$dir/settings.json"
    mkdir -p "$dir" || return 1
    current='{}'
    if [ -s "$settings" ]; then
        current="$(cat "$settings")" || return 1
    fi
    tmp="$(mktemp "$settings.XXXXXX")" || return 1
    if ! printf '%s' "$current" | jq --arg cmd "$self" '
        def entry($action): {hooks: [{type: "command", command: ($cmd + " " + $action), timeout: 10}]};
        def strip: map(.hooks |= map(select((.command // "") | startswith($cmd + " ") | not)))
                   | map(select(.hooks | length > 0));
        .hooks = ((.hooks // {}) | with_entries(.value |= strip) | with_entries(select(.value | length > 0)))
        | .hooks.Stop += [entry("stop")]
        | .hooks.UserPromptSubmit += [entry("release")]
        | .hooks.StopFailure += [entry("release")]
        | .hooks.SessionEnd += [entry("release")]
        | .hooks.Notification += [entry("release") + {matcher: "permission_prompt|elicitation_dialog|elicitation_url_dialog"}]
    ' >"$tmp"; then
        rm -f "$tmp"
        echo "rig-herdr-claude-hook: cannot update $settings (is it valid JSON?)" >&2
        return 1
    fi
    mv "$tmp" "$settings"
}

# background_summary prints e.g. "2 background tasks (shell, subagent)" for a
# Stop hook input with background work, and nothing when there is none.
background_summary() {
    jq -r '
        [(.background_tasks // [])[] | (.type // "task")]
        | if length == 0 then empty
          else "\(length) background task\(if length > 1 then "s" else "" end) (\(unique | join(", ")))"
          end
    ' 2>/dev/null
}

herdr_call() {
    timeout 3 "${HERDR_BIN_PATH:-herdr}" "$@" >/dev/null 2>&1 || true
}

action="${1:-}"
case "$action" in
    install) install_hooks; exit $? ;;
    stop | release) ;;
    *) exit 0 ;;
esac

input="$(cat 2>/dev/null)" || input=""

[ "${HERDR_ENV:-}" = "1" ] || exit 0
[ -n "${HERDR_PANE_ID:-}" ] || exit 0
[ -n "${HERDR_SOCKET_PATH:-}" ] || exit 0

# herdr ignores a report whose seq is not higher than the last it accepted
# from this source, so a nanosecond clock keeps late reports from winning.
seq="$(date +%s%N)"

summary=""
if [ "$action" = "stop" ]; then
    summary="$(printf '%s' "$input" | background_summary)" || summary=""
fi

if [ -n "$summary" ]; then
    herdr_call pane report-agent "$HERDR_PANE_ID" --source "$SOURCE" --agent "$AGENT" \
        --state working --message "$summary" --seq "$seq"
else
    herdr_call pane release-agent "$HERDR_PANE_ID" --source "$SOURCE" --agent "$AGENT" --seq "$seq"
fi
exit 0
