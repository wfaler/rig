package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/docker/docker/api/types/container"
	"github.com/moby/term"
)

// Attach connects stdin/stdout to a container with TTY support. Extra
// environment variables (env) are injected into the exec session; this is used
// to pass per-session herdr context that must not be baked into the cached
// container at create time.
func (c *Client) Attach(ctx context.Context, containerID string, command []string, env map[string]string) error {
	// Create exec instance to run the command
	execConfig := container.ExecOptions{
		Cmd:          command,
		Env:          envMapToSlice(env),
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	execResp, err := c.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	// Get terminal file descriptor
	fd := os.Stdin.Fd()

	// Check if stdin is a terminal
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}

	// Set terminal to raw mode
	oldState, err := term.SetRawTerminal(fd)
	if err != nil {
		return fmt.Errorf("setting raw terminal: %w", err)
	}
	defer func() {
		_ = term.RestoreTerminal(fd, oldState)
	}()

	// Attach to exec instance
	attachResp, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{
		Tty: true,
	})
	if err != nil {
		return fmt.Errorf("attaching to exec: %w", err)
	}
	defer attachResp.Close()

	// Handle terminal resize
	resizeCh := make(chan struct{})
	go c.handleResize(ctx, execResp.ID, resizeCh)

	// Initial resize
	c.resizeExecTTY(ctx, execResp.ID)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-sigCh:
				c.resizeExecTTY(ctx, execResp.ID)
			case <-ctx.Done():
				return
			}
		}
	}()
	defer signal.Stop(sigCh)

	// Copy I/O streams
	errCh := make(chan error, 2)

	// Copy container output to stdout
	go func() {
		_, err := io.Copy(os.Stdout, attachResp.Reader)
		errCh <- err
	}()

	// Copy stdin to container
	go func() {
		_, err := io.Copy(attachResp.Conn, os.Stdin)
		errCh <- err
	}()

	// Wait for exec to complete or context cancellation
	select {
	case err := <-errCh:
		if err != nil && err != io.EOF {
			return fmt.Errorf("I/O error: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	// Wait a moment for the other goroutine
	select {
	case <-errCh:
	default:
	}

	return nil
}

// envMapToSlice converts an env map to a sorted "KEY=VALUE" slice. Sorting
// keeps the output deterministic (useful for tests and reproducibility).
func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(env))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

// handleResize monitors for resize requests
func (c *Client) handleResize(ctx context.Context, execID string, resizeCh chan struct{}) {
	for {
		select {
		case <-resizeCh:
			c.resizeExecTTY(ctx, execID)
		case <-ctx.Done():
			return
		}
	}
}

// resizeExecTTY resizes the exec TTY to match the current terminal size
func (c *Client) resizeExecTTY(ctx context.Context, execID string) {
	fd := os.Stdout.Fd()
	ws, err := term.GetWinsize(fd)
	if err != nil {
		return
	}

	_ = c.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: uint(ws.Height),
		Width:  uint(ws.Width),
	})
}
