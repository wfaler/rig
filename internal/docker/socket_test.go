package docker

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEnv builds a getenv function backed by a map.
func fakeEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// usableSet builds a socket predicate that accepts exactly the given paths.
func usableSet(paths ...string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return func(path string) bool { return set[path] }
}

func TestResolveHostDockerSocket(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		env      map[string]string
		usable   []string
		expected string
	}{
		{
			name:     "linux docker default socket",
			goos:     "linux",
			usable:   []string{"/var/run/docker.sock"},
			expected: "/var/run/docker.sock",
		},
		{
			name:     "linux rootless podman via DOCKER_HOST",
			goos:     "linux",
			env:      map[string]string{"DOCKER_HOST": "unix:///run/user/1000/podman/podman.sock"},
			usable:   []string{"/run/user/1000/podman/podman.sock"},
			expected: "/run/user/1000/podman/podman.sock",
		},
		{
			name: "linux rootless podman skips unusable docker.sock symlink",
			goos: "linux",
			// The reported failure: /var/run/docker.sock links to the
			// root-owned /run/podman/podman.sock and cannot be statted.
			env:      map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"},
			usable:   []string{"/run/user/1000/podman/podman.sock"},
			expected: "/run/user/1000/podman/podman.sock",
		},
		{
			name:     "linux rootless docker under XDG_RUNTIME_DIR",
			goos:     "linux",
			env:      map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"},
			usable:   []string{"/run/user/1000/docker.sock"},
			expected: "/run/user/1000/docker.sock",
		},
		{
			name:     "linux rootful podman",
			goos:     "linux",
			usable:   []string{"/run/podman/podman.sock"},
			expected: "/run/podman/podman.sock",
		},
		{
			name:     "linux prefers DOCKER_HOST over probing",
			goos:     "linux",
			env:      map[string]string{"DOCKER_HOST": "unix:///custom/engine.sock", "XDG_RUNTIME_DIR": "/run/user/1000"},
			usable:   []string{"/custom/engine.sock", "/var/run/docker.sock", "/run/user/1000/podman/podman.sock"},
			expected: "/custom/engine.sock",
		},
		{
			name:     "linux falls back to probing when DOCKER_HOST socket is gone",
			goos:     "linux",
			env:      map[string]string{"DOCKER_HOST": "unix:///stale/engine.sock", "XDG_RUNTIME_DIR": "/run/user/1000"},
			usable:   []string{"/run/user/1000/podman/podman.sock"},
			expected: "/run/user/1000/podman/podman.sock",
		},
		{
			name:     "linux remote DOCKER_HOST uses conventional path",
			goos:     "linux",
			env:      map[string]string{"DOCKER_HOST": "tcp://192.168.1.10:2375"},
			usable:   nil,
			expected: "/var/run/docker.sock",
		},
		{
			name:     "linux ssh DOCKER_HOST uses conventional path",
			goos:     "linux",
			env:      map[string]string{"DOCKER_HOST": "ssh://user@remote"},
			usable:   nil,
			expected: "/var/run/docker.sock",
		},
		{
			name:     "linux with no reachable socket",
			goos:     "linux",
			env:      map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"},
			usable:   nil,
			expected: "",
		},
		{
			name: "darwin always uses the conventional path",
			goos: "darwin",
			// Host-side paths say nothing about the engine VM's filesystem.
			env:      map[string]string{"DOCKER_HOST": "unix:///Users/dev/.local/share/containers/podman/machine/podman.sock"},
			usable:   nil,
			expected: "/var/run/docker.sock",
		},
		{
			name:     "windows uses the conventional path",
			goos:     "windows",
			expected: "/var/run/docker.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveHostDockerSocket(tt.goos, fakeEnv(tt.env), usableSet(tt.usable...))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveHostDockerSocketCandidateOrder(t *testing.T) {
	// When several sockets are present, the conventional path wins so hosts
	// running both Docker and Podman keep their existing behaviour.
	env := fakeEnv(map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"})
	usable := usableSet("/var/run/docker.sock", "/run/user/1000/docker.sock", "/run/user/1000/podman/podman.sock", "/run/podman/podman.sock")
	assert.Equal(t, "/var/run/docker.sock", resolveHostDockerSocket("linux", env, usable))

	// Rootless Docker outranks rootless Podman, which outranks rootful Podman.
	usable = usableSet("/run/user/1000/docker.sock", "/run/user/1000/podman/podman.sock", "/run/podman/podman.sock")
	assert.Equal(t, "/run/user/1000/docker.sock", resolveHostDockerSocket("linux", env, usable))

	usable = usableSet("/run/user/1000/podman/podman.sock", "/run/podman/podman.sock")
	assert.Equal(t, "/run/user/1000/podman/podman.sock", resolveHostDockerSocket("linux", env, usable))
}

func TestUnixSocketPath(t *testing.T) {
	tests := []struct {
		name       string
		dockerHost string
		expected   string
		ok         bool
	}{
		{"unix scheme", "unix:///var/run/docker.sock", "/var/run/docker.sock", true},
		{"podman rootless socket", "unix:///run/user/1000/podman/podman.sock", "/run/user/1000/podman/podman.sock", true},
		{"http+unix scheme", "http+unix:///run/podman/podman.sock", "/run/podman/podman.sock", true},
		{"schemeless unix prefix", "unix:/var/run/docker.sock", "/var/run/docker.sock", true},
		{"empty", "", "", false},
		{"tcp", "tcp://127.0.0.1:2375", "", false},
		{"ssh", "ssh://user@host", "", false},
		{"npipe", "npipe:////./pipe/docker_engine", "", false},
		{"unix scheme with relative path", "unix://relative/path.sock", "", false},
		{"unix scheme with empty path", "unix://", "", false},
		{"bare path is not a URL", "/var/run/docker.sock", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := unixSocketPath(tt.dockerHost)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.expected, path)
		})
	}
}

func TestIsUsableSocket(t *testing.T) {
	dir := t.TempDir()

	sockPath := filepath.Join(dir, "engine.sock")
	listener, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer listener.Close()

	regularFile := filepath.Join(dir, "not-a-socket")
	require.NoError(t, os.WriteFile(regularFile, []byte("x"), 0o600))

	symlinkToSocket := filepath.Join(dir, "linked.sock")
	require.NoError(t, os.Symlink(sockPath, symlinkToSocket))

	danglingLink := filepath.Join(dir, "dangling.sock")
	require.NoError(t, os.Symlink(filepath.Join(dir, "missing.sock"), danglingLink))

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"live socket", sockPath, true},
		{"symlink to socket", symlinkToSocket, true},
		{"regular file", regularFile, false},
		{"directory", dir, false},
		{"missing path", filepath.Join(dir, "absent.sock"), false},
		{"dangling symlink", danglingLink, false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isUsableSocket(tt.path))
		})
	}
}

func TestIsUsableSocketUnreadableParentDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	// Mirrors rootless Podman on Linux: /var/run/docker.sock points into a
	// root-owned 0700 directory, so the socket cannot be statted or mounted.
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	require.NoError(t, os.Mkdir(private, 0o700))

	sockPath := filepath.Join(private, "engine.sock")
	listener, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer listener.Close()

	link := filepath.Join(dir, "docker.sock")
	require.NoError(t, os.Symlink(sockPath, link))
	require.True(t, isUsableSocket(link), "precondition: socket is reachable while the directory is traversable")

	require.NoError(t, os.Chmod(private, 0o000))
	t.Cleanup(func() { _ = os.Chmod(private, 0o700) })

	assert.False(t, isUsableSocket(link), "socket behind an unreadable directory must be rejected")
	assert.False(t, isUsableSocket(sockPath))
}

func TestHostDockerSocketDoesNotPanic(t *testing.T) {
	// Exercises the real environment/filesystem wiring.
	assert.NotPanics(t, func() { HostDockerSocket() })
}
