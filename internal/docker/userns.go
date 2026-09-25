package docker

import (
	"context"
	"strings"
)

// EnvUsernsMode is the container-side environment variable recording the user
// namespace mode the container was created with. Engines do not report it back
// faithfully on inspect (Podman shows "keep-id" as "private"), so it is baked
// into the env to detect containers that need recreating when it changes.
const EnvUsernsMode = "RIG_USERNS_MODE"

// UsernsKeepID maps the invoking host user to the same uid inside the
// container, so the image's non-root developer user can write to /workspace.
const UsernsKeepID = "keep-id"

// DesiredUsernsMode returns the user namespace mode rig containers should be
// created with on the connected engine, or "" for the engine default.
//
// Under rootless Podman the host user is root inside the container by default,
// so the developer user cannot write to the bind-mounted workspace; keep-id
// fixes that per container without forcing it on every container via
// containers.conf (which breaks images that need root at startup, e.g.
// postgres). Docker and rootful Podman need no mapping. Detection failures fall
// back to the engine default.
func (c *Client) DesiredUsernsMode(ctx context.Context) string {
	version, err := c.cli.ServerVersion(ctx)
	if err != nil {
		return ""
	}
	info, err := c.cli.Info(ctx)
	if err != nil {
		return ""
	}
	components := make([]string, 0, len(version.Components))
	for _, comp := range version.Components {
		components = append(components, comp.Name)
	}
	return usernsModeFor(components, info.SecurityOptions)
}

// usernsModeFor picks the user namespace mode from the engine's component
// names and security options.
func usernsModeFor(components []string, securityOptions []string) string {
	podman := false
	for _, name := range components {
		if strings.Contains(strings.ToLower(name), "podman") {
			podman = true
			break
		}
	}
	if !podman {
		return ""
	}
	for _, opt := range securityOptions {
		if opt == "name=rootless" {
			return UsernsKeepID
		}
	}
	return ""
}
