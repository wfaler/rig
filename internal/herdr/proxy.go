package herdr

import (
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"runtime"
)

// EnvProxyPort is the container-side environment variable carrying the host
// TCP port that bridges to the herdr control socket. When set, the container
// entrypoint starts a socat process that presents ContainerSocketPath and
// forwards connections to host.docker.internal:<port>. It is not a herdr
// variable.
const EnvProxyPort = "RIG_HERDR_PROXY_PORT"

// Deterministic per-project bridge ports live in this range, chosen to avoid
// the ephemeral range on both macOS (49152+) and common service ports.
const (
	proxyPortBase  = 42000
	proxyPortRange = 7000
)

// UseTCPBridge reports whether the herdr socket must be bridged over TCP
// instead of bind-mounted. On macOS the Docker VM shares the socket file via
// virtiofs but connections cannot cross the VM boundary (connect() is
// refused), so the socket is bridged: rig listens on a loopback TCP port on
// the host and socat re-exposes it as a unix socket inside the container. On
// Linux the direct bind mount works.
func UseTCPBridge() bool {
	return runtime.GOOS == "darwin"
}

// ProxyPort returns a deterministic host TCP port for a project's herdr
// bridge. Deriving it from the project name keeps the port stable across
// container recreation (it is baked into the container's env at create time)
// while avoiding collisions between concurrently running projects.
func ProxyPort(projectName string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(projectName))
	return proxyPortBase + int(h.Sum32()%proxyPortRange)
}

// StartProxy listens on 127.0.0.1:port and forwards each TCP connection to
// the unix socket at socketPath. It serves in the background for the lifetime
// of the process; connections are bridged bidirectionally with independent
// half-close so newline-delimited request/response exchanges work unmodified.
func StartProxy(socketPath string, port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("listening on herdr bridge port %d: %w", port, err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go bridgeConn(conn, socketPath)
		}
	}()
	return nil
}

// bridgeConn forwards one TCP connection to the herdr unix socket and back,
// propagating EOF in each direction via half-close.
func bridgeConn(conn net.Conn, socketPath string) {
	defer func() { _ = conn.Close() }()
	sock, err := net.Dial("unix", socketPath)
	if err != nil {
		return
	}
	defer func() { _ = sock.Close() }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(sock, conn)
		if uc, ok := sock.(*net.UnixConn); ok {
			_ = uc.CloseWrite()
		}
	}()
	_, _ = io.Copy(conn, sock)
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}
	<-done
}
