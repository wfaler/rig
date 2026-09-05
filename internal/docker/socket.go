package docker

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ContainerDockerSocket is the path the engine socket is mounted at inside the
// container. The image bakes DOCKER_HOST and the testcontainers overrides to
// this path, so it stays fixed no matter where the socket lives outside.
const ContainerDockerSocket = "/var/run/docker.sock"

// unixSchemes are the DOCKER_HOST URL schemes that address a local socket.
var unixSchemes = []string{"unix://", "http+unix://", "unix:"}

// HostDockerSocket returns the path to bind-mount at ContainerDockerSocket so
// tooling inside the container (testcontainers, docker CLI) can reach the
// engine, or "" when no usable socket can be found.
func HostDockerSocket() string {
	return resolveHostDockerSocket(runtime.GOOS, os.Getenv, isUsableSocket)
}

// resolveHostDockerSocket picks the bind source for the engine socket.
//
// The bind source is resolved by whoever runs the engine, so it is only worth
// probing the local filesystem when the engine shares it. That holds for
// Docker and Podman on Linux, where the socket location varies a lot: rootful
// Docker uses /var/run/docker.sock, rootless Docker and rootless Podman keep
// theirs under $XDG_RUNTIME_DIR, and rootful Podman uses
// /run/podman/podman.sock. Under rootless Podman, /var/run/docker.sock is
// commonly a symlink to the root-owned /run/podman/podman.sock, which the
// engine cannot statfs as the invoking user — mounting it fails container
// creation with an opaque "permission denied", so unusable candidates are
// skipped rather than mounted blindly.
//
// Everywhere else — Docker Desktop, Podman machine, Colima, or a remote
// DOCKER_HOST — the engine runs in a VM or on another host with its own
// filesystem, and local paths say nothing about it. There the conventional
// ContainerDockerSocket is the only meaningful answer: it is what those VM
// images expose (Podman machine's docker-compat symlink included), and it is
// the path that has always worked on macOS.
func resolveHostDockerSocket(goos string, getenv func(string) string, usable func(string) bool) string {
	if goos != "linux" {
		return ContainerDockerSocket
	}

	dockerHost := getenv("DOCKER_HOST")
	if path, ok := unixSocketPath(dockerHost); ok {
		if usable(path) {
			return path
		}
	} else if dockerHost != "" {
		// A remote engine (tcp://, ssh://): its socket path is not ours to
		// probe, so fall back to the conventional location.
		return ContainerDockerSocket
	}

	for _, candidate := range linuxSocketCandidates(getenv) {
		if usable(candidate) {
			return candidate
		}
	}
	return ""
}

// linuxSocketCandidates lists well-known engine socket locations on Linux, in
// preference order: the conventional path, then rootless Docker and rootless
// Podman under $XDG_RUNTIME_DIR, then rootful Podman.
func linuxSocketCandidates(getenv func(string) string) []string {
	candidates := []string{ContainerDockerSocket}
	if runtimeDir := getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates,
			filepath.Join(runtimeDir, "docker.sock"),
			filepath.Join(runtimeDir, "podman", "podman.sock"),
		)
	}
	return append(candidates, "/run/podman/podman.sock")
}

// unixSocketPath extracts the filesystem path from a DOCKER_HOST value that
// addresses a unix socket. It reports false for empty, remote (tcp://, ssh://)
// or malformed values.
func unixSocketPath(dockerHost string) (string, bool) {
	for _, scheme := range unixSchemes {
		if !strings.HasPrefix(dockerHost, scheme) {
			continue
		}
		path := strings.TrimPrefix(dockerHost, scheme)
		if !strings.HasPrefix(path, "/") {
			return "", false
		}
		return path, true
	}
	return "", false
}

// isUsableSocket reports whether path resolves to a socket the current user
// can reach. Stat follows symlinks, so an unreadable target directory (or a
// dangling link) correctly rules the candidate out.
func isUsableSocket(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}
