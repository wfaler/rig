package herdr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// mapLookup builds a LookupFunc from a map.
func mapLookup(m map[string]string) LookupFunc {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Info
	}{
		{
			name: "not in herdr",
			env:  map[string]string{},
			want: Info{Active: false},
		},
		{
			name: "full herdr context",
			env: map[string]string{
				EnvActive:      "1",
				EnvSocketPath:  "/home/u/.config/herdr/herdr.sock",
				EnvPaneID:      "pane-1",
				EnvWorkspaceID: "ws-1",
				EnvTabID:       "tab-1",
				EnvAgent:       "claude",
			},
			want: Info{
				Active:      true,
				SocketPath:  "/home/u/.config/herdr/herdr.sock",
				PaneID:      "pane-1",
				WorkspaceID: "ws-1",
				TabID:       "tab-1",
				Agent:       "claude",
			},
		},
		{
			name: "active flag not exactly 1 is inactive",
			env:  map[string]string{EnvActive: "true"},
			want: Info{Active: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(mapLookup(tt.env))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsActive(t *testing.T) {
	assert.True(t, Info{Active: true, SocketPath: "/s"}.IsActive())
	assert.False(t, Info{Active: true, SocketPath: ""}.IsActive(), "no socket means not usable")
	assert.False(t, Info{Active: false, SocketPath: "/s"}.IsActive())
}

func TestContainerEnv(t *testing.T) {
	info := Info{
		Active:      true,
		SocketPath:  "/home/u/.config/herdr/herdr.sock",
		PaneID:      "pane-1",
		WorkspaceID: "ws-1",
		TabID:       "tab-1",
		Agent:       "gemini", // detected value should be overridden by arg
	}
	env := info.ContainerEnv("claude")

	assert.Equal(t, "1", env[EnvActive])
	assert.Equal(t, ContainerSocketPath, env[EnvSocketPath], "socket must be remapped to in-container path")
	assert.Equal(t, "claude", env[EnvAgent], "agent arg overrides detected value")
	assert.Equal(t, "pane-1", env[EnvPaneID])
	assert.Equal(t, "ws-1", env[EnvWorkspaceID])
	assert.Equal(t, "tab-1", env[EnvTabID])
}

func TestContainerEnvOmitsEmptyContextIDs(t *testing.T) {
	info := Info{Active: true, SocketPath: "/s"}
	env := info.ContainerEnv("claude")

	assert.Equal(t, "1", env[EnvActive])
	assert.Equal(t, "claude", env[EnvAgent])
	_, hasPane := env[EnvPaneID]
	_, hasWs := env[EnvWorkspaceID]
	_, hasTab := env[EnvTabID]
	assert.False(t, hasPane)
	assert.False(t, hasWs)
	assert.False(t, hasTab)
}

func TestNeedsReexec(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"not in herdr", map[string]string{}, false},
		{"in herdr, agent unset", map[string]string{EnvActive: "1"}, true},
		{"agent already set", map[string]string{EnvActive: "1", EnvAgent: "claude"}, false},
		{"agent set empty is treated as unset", map[string]string{EnvActive: "1", EnvAgent: ""}, true},
		{"already re-exec'd", map[string]string{EnvActive: "1", reexecSentinel: "1"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NeedsReexec(mapLookup(tt.env)))
		})
	}
}

func TestReexecEnv(t *testing.T) {
	current := []string{
		"PATH=/usr/bin",
		"HERDR_ENV=1",
		"HERDR_AGENT=stale",    // should be replaced
		"RIG_HERDR_REEXEC=old", // should be replaced
		"HOME=/home/u",
	}
	out := ReexecEnv(current, "claude")

	// Preserves unrelated vars.
	assert.Contains(t, out, "PATH=/usr/bin")
	assert.Contains(t, out, "HERDR_ENV=1")
	assert.Contains(t, out, "HOME=/home/u")

	// Sets exactly one HERDR_AGENT and one sentinel, with the new values.
	assert.Contains(t, out, "HERDR_AGENT=claude")
	assert.Contains(t, out, "RIG_HERDR_REEXEC=1")
	assert.Equal(t, 1, count(out, "HERDR_AGENT="))
	assert.Equal(t, 1, count(out, "RIG_HERDR_REEXEC="))
	assert.NotContains(t, out, "HERDR_AGENT=stale")

	// After re-exec, NeedsReexec against the new env is false.
	assert.False(t, NeedsReexec(sliceLookup(out)))
}

func count(kvs []string, prefix string) int {
	n := 0
	for _, kv := range kvs {
		if len(kv) >= len(prefix) && kv[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

// sliceLookup turns a KEY=VALUE slice into a LookupFunc.
func sliceLookup(kvs []string) LookupFunc {
	m := map[string]string{}
	for _, kv := range kvs {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return mapLookup(m)
}
