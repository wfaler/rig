package dockerfile

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHerdr is a herdr stand-in that logs each invocation's arguments as one
// line, and exits with HERDR_FAKE_EXIT so failure handling can be tested.
const fakeHerdr = `#!/bin/sh
printf '%s\n' "$*" >> "$HERDR_FAKE_LOG"
exit "${HERDR_FAKE_EXIT:-0}"
`

// requireHookTools skips the test when the hook's runtime dependencies are
// missing, as they are on some developer machines (the image always has them).
func requireHookTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"sh", "jq", "timeout", "readlink", "mktemp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
}

// writeExecutable writes content to dir/name with execute permission.
func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

type hookResult struct {
	calls  []string // one line of arguments per herdr invocation
	stdout string
	err    error
}

// runHook runs the embedded hook with the given action, stdin and extra
// environment, against a fake herdr binary.
func runHook(t *testing.T, action, stdin string, env map[string]string) hookResult {
	t.Helper()
	dir := t.TempDir()
	hook := writeExecutable(t, dir, herdrClaudeHookFile, herdrClaudeHook)
	bin := writeExecutable(t, dir, "herdr", fakeHerdr)
	log := filepath.Join(dir, "herdr.log")

	cmd := exec.Command(hook, action)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + dir,
		"HERDR_BIN_PATH=" + bin,
		"HERDR_FAKE_LOG=" + log,
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdin = strings.NewReader(stdin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()

	var calls []string
	if data, readErr := os.ReadFile(log); readErr == nil {
		calls = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	}
	return hookResult{calls: calls, stdout: stdout.String(), err: err}
}

// inHerdr is the environment herdr sets inside a managed pane.
var inHerdr = map[string]string{
	"HERDR_ENV":         "1",
	"HERDR_PANE_ID":     "wB:p1",
	"HERDR_SOCKET_PATH": "/run/herdr/herdr.sock",
}

func withEnv(base map[string]string, overrides map[string]string) map[string]string {
	env := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		env[k] = v
	}
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

func TestHerdrClaudeHookReporting(t *testing.T) {
	requireHookTools(t)

	const (
		release = "pane release-agent wB:p1 --source rig:claude --agent claude --seq "
		working = "pane report-agent wB:p1 --source rig:claude --agent claude --state working --message "
	)
	tests := []struct {
		name       string
		action     string
		stdin      string
		env        map[string]string
		wantPrefix string // expected prefix of the single herdr call; "" means no call
	}{
		{
			name:       "stop with background tasks reports working with a summary",
			action:     "stop",
			stdin:      `{"hook_event_name":"Stop","background_tasks":[{"id":"a","type":"shell"},{"id":"b","type":"subagent"},{"id":"c","type":"shell"}]}`,
			env:        inHerdr,
			wantPrefix: working + "3 background tasks (shell, subagent) --seq ",
		},
		{
			name:       "stop with one background task uses the singular",
			action:     "stop",
			stdin:      `{"background_tasks":[{"id":"a","type":"monitor"}]}`,
			env:        inHerdr,
			wantPrefix: working + "1 background task (monitor) --seq ",
		},
		{
			name:       "stop labels a task without a type",
			action:     "stop",
			stdin:      `{"background_tasks":[{"id":"a"}]}`,
			env:        inHerdr,
			wantPrefix: working + "1 background task (task) --seq ",
		},
		{
			name:       "stop with no background tasks releases",
			action:     "stop",
			stdin:      `{"hook_event_name":"Stop","background_tasks":[]}`,
			env:        inHerdr,
			wantPrefix: release,
		},
		{
			name:       "stop without the background_tasks field releases",
			action:     "stop",
			stdin:      `{"hook_event_name":"Stop"}`,
			env:        inHerdr,
			wantPrefix: release,
		},
		{
			name:       "stop with unparseable input releases",
			action:     "stop",
			stdin:      `not json`,
			env:        inHerdr,
			wantPrefix: release,
		},
		{
			name:       "stop with empty input releases",
			action:     "stop",
			stdin:      "",
			env:        inHerdr,
			wantPrefix: release,
		},
		{
			name:       "release ignores background tasks in its input",
			action:     "release",
			stdin:      `{"background_tasks":[{"id":"a","type":"shell"}]}`,
			env:        inHerdr,
			wantPrefix: release,
		},
		{
			name:   "outside herdr nothing is called",
			action: "stop",
			stdin:  `{"background_tasks":[{"id":"a","type":"shell"}]}`,
			env:    map[string]string{},
		},
		{
			name:   "HERDR_ENV other than 1 calls nothing",
			action: "release",
			env:    withEnv(inHerdr, map[string]string{"HERDR_ENV": "0"}),
		},
		{
			name:   "missing pane id calls nothing",
			action: "release",
			env:    withEnv(inHerdr, map[string]string{"HERDR_PANE_ID": ""}),
		},
		{
			name:   "missing socket path calls nothing",
			action: "release",
			env:    withEnv(inHerdr, map[string]string{"HERDR_SOCKET_PATH": ""}),
		},
		{
			name:   "unknown action calls nothing",
			action: "bogus",
			env:    inHerdr,
		},
		{
			name:   "no action calls nothing",
			action: "",
			env:    inHerdr,
		},
		{
			name:       "a failing herdr does not fail the hook",
			action:     "stop",
			stdin:      `{"background_tasks":[{"id":"a","type":"shell"}]}`,
			env:        withEnv(inHerdr, map[string]string{"HERDR_FAKE_EXIT": "1"}),
			wantPrefix: working,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runHook(t, tt.action, tt.stdin, tt.env)
			require.NoError(t, res.err, "hook must always exit 0")
			assert.Empty(t, res.stdout, "hook must print nothing: UserPromptSubmit output reaches Claude's context")
			if tt.wantPrefix == "" {
				assert.Empty(t, res.calls)
				return
			}
			require.Len(t, res.calls, 1)
			assert.True(t, strings.HasPrefix(res.calls[0], tt.wantPrefix), "got %q, want prefix %q", res.calls[0], tt.wantPrefix)
		})
	}
}

func TestHerdrClaudeHookSeqIncreases(t *testing.T) {
	requireHookTools(t)
	seqOf := func() string {
		res := runHook(t, "release", "", inHerdr)
		require.NoError(t, res.err)
		require.Len(t, res.calls, 1)
		fields := strings.Fields(res.calls[0])
		return fields[len(fields)-1]
	}
	first, second := seqOf(), seqOf()
	require.Len(t, first, len(second), "nanosecond timestamps compare correctly as strings only at equal length")
	assert.Less(t, first, second, "herdr drops reports whose seq does not increase")
}

// installHook copies the hook into dir and runs its install subcommand with
// HOME=dir, returning the script path and any error.
func installHook(t *testing.T, dir string) (string, error) {
	t.Helper()
	hook := writeExecutable(t, dir, herdrClaudeHookFile, herdrClaudeHook)
	cmd := exec.Command(hook, "install")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("install output: %s", out)
	}
	resolved, evalErr := filepath.EvalSymlinks(hook)
	require.NoError(t, evalErr)
	return resolved, err
}

// claudeSettings reads and parses dir/.claude/settings.json.
func claudeSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	return settings
}

// hookCommands returns the commands registered for an event, with their matchers.
func hookCommands(t *testing.T, settings map[string]any, event string) []string {
	t.Helper()
	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "settings must have a hooks object")
	groups, _ := hooks[event].([]any)
	var commands []string
	for _, g := range groups {
		group := g.(map[string]any)
		matcher, _ := group["matcher"].(string)
		for _, h := range group["hooks"].([]any) {
			commands = append(commands, matcher+"|"+h.(map[string]any)["command"].(string))
		}
	}
	return commands
}

func TestHerdrClaudeHookInstall(t *testing.T) {
	requireHookTools(t)

	t.Run("creates settings with every hooked event", func(t *testing.T) {
		dir := t.TempDir()
		hook, err := installHook(t, dir)
		require.NoError(t, err)
		settings := claudeSettings(t, dir)

		expected := map[string][]string{
			"Stop":             {"|" + hook + " stop"},
			"UserPromptSubmit": {"|" + hook + " release"},
			"StopFailure":      {"|" + hook + " release"},
			"SessionEnd":       {"|" + hook + " release"},
			"Notification":     {"permission_prompt|elicitation_dialog|elicitation_url_dialog|" + hook + " release"},
		}
		for event, want := range expected {
			assert.Equal(t, want, hookCommands(t, settings, event), event)
		}
		assert.Len(t, settings["hooks"], len(expected))
	})

	t.Run("keeps existing settings and hooks", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0o755))
		existing := `{"theme":"dark","hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify-send done"}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"audit"}]}]}}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(existing), 0o644))

		hook, err := installHook(t, dir)
		require.NoError(t, err)
		settings := claudeSettings(t, dir)

		assert.Equal(t, "dark", settings["theme"])
		assert.Equal(t, []string{"|notify-send done", "|" + hook + " stop"}, hookCommands(t, settings, "Stop"))
		assert.Equal(t, []string{"Bash|audit"}, hookCommands(t, settings, "PreToolUse"))
	})

	t.Run("is idempotent", func(t *testing.T) {
		dir := t.TempDir()
		_, err := installHook(t, dir)
		require.NoError(t, err)
		first, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
		require.NoError(t, err)

		_, err = installHook(t, dir)
		require.NoError(t, err)
		second, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
		require.NoError(t, err)
		assert.JSONEq(t, string(first), string(second))
	})

	t.Run("replaces entries it no longer installs", func(t *testing.T) {
		dir := t.TempDir()
		hook := filepath.Join(dir, herdrClaudeHookFile)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0o755))
		stale := `{"hooks":{"SubagentStop":[{"hooks":[{"type":"command","command":"` + hook + ` stop"}]}]}}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(stale), 0o644))

		_, err := installHook(t, dir)
		require.NoError(t, err)
		hooks := claudeSettings(t, dir)["hooks"].(map[string]any)
		assert.NotContains(t, hooks, "SubagentStop", "an event left with no hooks is removed")
	})

	t.Run("leaves invalid settings untouched and fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".claude", "settings.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))

		_, err := installHook(t, dir)
		require.Error(t, err)
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "{not json", string(data))
		entries, readErr := os.ReadDir(filepath.Dir(path))
		require.NoError(t, readErr)
		assert.Len(t, entries, 1, "no temporary file is left behind")
	})
}
