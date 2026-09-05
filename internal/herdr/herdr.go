// Package herdr provides detection of, and integration with, the herdr
// terminal agent multiplexer (https://herdr.dev).
//
// When rig runs inside a herdr-managed pane, herdr injects a set of
// environment variables (HERDR_ENV, HERDR_SOCKET_PATH, HERDR_PANE_ID, ...)
// and exposes a Unix-socket control API. This package reads those variables
// and assembles the environment needed to make a containerized agent (e.g.
// Claude Code) able to talk back to the herdr socket, plus the host-side
// environment needed for herdr to detect the agent through the container.
package herdr

// Environment variable names set by herdr inside a managed pane.
const (
	EnvActive      = "HERDR_ENV"          // "1" when inside a herdr pane
	EnvSocketPath  = "HERDR_SOCKET_PATH"  // path to the herdr control socket
	EnvPaneID      = "HERDR_PANE_ID"      // current pane identifier
	EnvWorkspaceID = "HERDR_WORKSPACE_ID" // current workspace identifier
	EnvTabID       = "HERDR_TAB_ID"       // current tab identifier
	EnvAgent       = "HERDR_AGENT"        // agent manifest herdr should use for detection
)

// reexecSentinel guards against an infinite re-exec loop when rig re-launches
// itself with HERDR_AGENT set. It is not a herdr variable.
const reexecSentinel = "RIG_HERDR_REEXEC"

// ContainerSocketPath is the fixed path the host herdr socket is bind-mounted
// to inside the container. Using a stable path keeps HERDR_SOCKET_PATH
// consistent regardless of where the socket lives on the host.
const ContainerSocketPath = "/run/herdr/herdr.sock"

// Info holds the herdr context detected from the environment.
type Info struct {
	Active      bool
	SocketPath  string
	PaneID      string
	WorkspaceID string
	TabID       string
	Agent       string
}

// LookupFunc mirrors os.LookupEnv and is injectable for testing.
type LookupFunc func(key string) (string, bool)

// Detect reads herdr context from the environment using the given lookup.
func Detect(lookup LookupFunc) Info {
	get := func(k string) string {
		v, _ := lookup(k)
		return v
	}
	active, _ := lookup(EnvActive)
	return Info{
		Active:      active == "1",
		SocketPath:  get(EnvSocketPath),
		PaneID:      get(EnvPaneID),
		WorkspaceID: get(EnvWorkspaceID),
		TabID:       get(EnvTabID),
		Agent:       get(EnvAgent),
	}
}

// IsActive reports whether rig is running inside a herdr pane with a usable
// socket. Both conditions are required for the container integration to work.
func (i Info) IsActive() bool {
	return i.Active && i.SocketPath != ""
}

// ContainerEnv returns the environment variables to inject into the container
// shell so the herdr skill and CLI work inside it. The socket path is remapped
// to its in-container mount point. agent overrides the detected HERDR_AGENT
// (rig sets this explicitly from config).
func (i Info) ContainerEnv(agent string) map[string]string {
	env := map[string]string{
		EnvActive:     "1",
		EnvSocketPath: ContainerSocketPath,
		EnvAgent:      agent,
	}
	if i.PaneID != "" {
		env[EnvPaneID] = i.PaneID
	}
	if i.WorkspaceID != "" {
		env[EnvWorkspaceID] = i.WorkspaceID
	}
	if i.TabID != "" {
		env[EnvTabID] = i.TabID
	}
	return env
}

// NeedsReexec reports whether rig should re-exec itself to expose
// HERDR_AGENT to herdr's process detection. herdr reads the foreground
// process's /proc/<pid>/environ, which only reflects the environment at exec
// time — so setting HERDR_AGENT via os.Setenv is not enough; we must re-exec.
// It returns false if we are not in herdr, if HERDR_AGENT is already set, or
// if we have already re-exec'd (sentinel present).
func NeedsReexec(lookup LookupFunc) bool {
	if active, _ := lookup(EnvActive); active != "1" {
		return false
	}
	if _, ok := lookup(reexecSentinel); ok {
		return false
	}
	if agent, ok := lookup(EnvAgent); ok && agent != "" {
		return false
	}
	return true
}

// ReexecEnv returns a new environment slice (KEY=VALUE form) based on current,
// with HERDR_AGENT set to agent and the re-exec sentinel added. Any existing
// HERDR_AGENT or sentinel entries are replaced so the result is unambiguous.
func ReexecEnv(current []string, agent string) []string {
	out := make([]string, 0, len(current)+2)
	for _, kv := range current {
		if hasKey(kv, EnvAgent) || hasKey(kv, reexecSentinel) {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, EnvAgent+"="+agent, reexecSentinel+"=1")
	return out
}

// hasKey reports whether a "KEY=VALUE" entry has the given key.
func hasKey(kv, key string) bool {
	return len(kv) > len(key) && kv[:len(key)] == key && kv[len(key)] == '='
}
