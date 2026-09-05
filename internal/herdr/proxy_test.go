package herdr

import (
	"bufio"
	"fmt"
	"net"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyPort(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		assert.Equal(t, ProxyPort("my-project"), ProxyPort("my-project"))
	})

	t.Run("within range", func(t *testing.T) {
		names := []string{"", "a", "rig", "some-long-project-name", "another"}
		for _, name := range names {
			port := ProxyPort(name)
			assert.GreaterOrEqual(t, port, proxyPortBase, "project %q", name)
			assert.Less(t, port, proxyPortBase+proxyPortRange, "project %q", name)
		}
	})

	t.Run("different projects usually differ", func(t *testing.T) {
		assert.NotEqual(t, ProxyPort("project-a"), ProxyPort("project-b"))
	})
}

func TestStartProxy(t *testing.T) {
	// Stand in for the herdr server: a unix socket echoing one line back
	// with a prefix, mimicking herdr's newline-delimited JSON exchange.
	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	ln, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				line, err := bufio.NewReader(c).ReadString('\n')
				if err != nil {
					return
				}
				_, _ = fmt.Fprintf(c, "pong:%s", line)
			}(conn)
		}
	}()

	// Pick a free port by asking the kernel, then hand it to StartProxy.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := probe.Addr().(*net.TCPAddr).Port
	require.NoError(t, probe.Close())

	require.NoError(t, StartProxy(socketPath, port))

	t.Run("bridges a request and response", func(t *testing.T) {
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		_, err = conn.Write([]byte("ping\n"))
		require.NoError(t, err)
		line, err := bufio.NewReader(conn).ReadString('\n')
		require.NoError(t, err)
		assert.Equal(t, "pong:ping\n", line)
	})

	t.Run("handles multiple sequential connections", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			require.NoError(t, err)
			_, err = conn.Write([]byte(fmt.Sprintf("req%d\n", i)))
			require.NoError(t, err)
			line, err := bufio.NewReader(conn).ReadString('\n')
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("pong:req%d\n", i), line)
			require.NoError(t, conn.Close())
		}
	})

	t.Run("port already in use returns error", func(t *testing.T) {
		err := StartProxy(socketPath, port)
		assert.Error(t, err)
	})
}
