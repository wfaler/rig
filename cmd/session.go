package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/wfaler/rig/internal/config"
	"github.com/wfaler/rig/internal/docker"
	"github.com/wfaler/rig/internal/dockerfile"
	"github.com/wfaler/rig/internal/herdr"
	"github.com/wfaler/rig/internal/project"
)

const configFileName = ".rig.yml"

// runSession handles the complete flow of loading config, building image,
// creating container, and attaching to run a command
func runSession(command []string) error {
	ctx := context.Background()

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Load config
	configPath := filepath.Join(cwd, configFileName)
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// When running inside herdr, re-exec rig with HERDR_AGENT set so herdr's
	// process detection (which reads the foreground process's environment) can
	// identify the agent through the container. This replaces the process, so
	// nothing after this point runs in the current invocation.
	if cfg.IsHerdrEnabled() {
		if err := maybeReexecForHerdr(cfg.GetHerdrAgent()); err != nil {
			return err
		}
	}

	// Detect herdr context for socket mounting and env injection.
	herdrInfo := herdr.Detect(os.LookupEnv)
	herdrActive := cfg.IsHerdrEnabled() && herdrInfo.IsActive()

	// Expand environment variables
	cfg.ExpandEnvVars()

	// Use configured shell if no command specified
	if len(command) == 0 {
		command = []string{"/bin/" + cfg.GetShell()}
	}

	// Generate project name and image reference
	projectName := project.GetProjectName(cwd)
	configHash, err := project.ComputeConfigHash(configPath)
	if err != nil {
		return fmt.Errorf("computing config hash: %w", err)
	}
	imageRef := project.ImageRef(projectName, configHash)
	containerName := project.ContainerName(projectName)

	// Create Docker client
	dockerClient, err := docker.New()
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer dockerClient.Close()

	// Check if image exists
	imageExists, err := dockerClient.ImageExists(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("checking image: %w", err)
	}

	if !imageExists {
		// Generate Dockerfile
		fmt.Printf("Building image %s...\n", imageRef)
		buildCtx, err := dockerfile.Generate(cfg)
		if err != nil {
			return fmt.Errorf("generating dockerfile: %w", err)
		}

		// Build image
		if err := dockerClient.BuildImage(ctx, buildCtx.Dockerfile, imageRef, buildCtx.ExtraFiles); err != nil {
			return fmt.Errorf("building image: %w", err)
		}
		fmt.Println("Image built successfully")
	}

	// Derive herdr wiring and the extra environment to inject into the
	// interactive shell. On Linux the host socket is bind-mounted directly;
	// on macOS unix sockets cannot cross the Docker VM boundary, so rig
	// bridges the socket over a loopback TCP port (the container entrypoint
	// re-exposes it as a unix socket via socat).
	herdrSocket := ""
	herdrProxyPort := 0
	var attachEnv map[string]string
	if herdrActive {
		if herdr.UseTCPBridge() {
			herdrProxyPort = herdr.ProxyPort(projectName)
			if err := herdr.StartProxy(herdrInfo.SocketPath, herdrProxyPort); err != nil {
				// Most likely another rig session for this project already
				// holds the port and is bridging; keep going either way.
				fmt.Printf("Note: %v (assuming an existing rig session is bridging)\n", err)
			}
		} else {
			herdrSocket = herdrInfo.SocketPath
		}
		attachEnv = herdrInfo.ContainerEnv(cfg.GetHerdrAgent())
	}

	// Always expose the host workdir in the interactive session too, so
	// containers created by older rig versions pick it up without recreation.
	if attachEnv == nil {
		attachEnv = map[string]string{}
	}
	attachEnv[docker.EnvHostWorkdir] = cwd

	// Find existing container
	containerID, err := dockerClient.FindContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("finding container: %w", err)
	}

	if containerID != "" {
		// Container exists - check its state and image
		running, err := dockerClient.IsContainerRunning(ctx, containerID)
		if err != nil {
			return fmt.Errorf("checking container status: %w", err)
		}

		currentImage, err := dockerClient.GetContainerImage(ctx, containerID)
		if err != nil {
			return fmt.Errorf("getting container image: %w", err)
		}

		// Herdr wiring (socket bind mount or bridge port env) is fixed at
		// create time, so a container built with a different herdr state must
		// be recreated to pick up the correct configuration.
		existingHerdrSocket, err := dockerClient.GetHerdrSocketHostPath(ctx, containerID)
		if err != nil {
			return fmt.Errorf("inspecting herdr mount: %w", err)
		}
		existingProxyPort, err := dockerClient.GetContainerEnvValue(ctx, containerID, herdr.EnvProxyPort)
		if err != nil {
			return fmt.Errorf("inspecting herdr bridge env: %w", err)
		}
		desiredProxyPort := ""
		if herdrProxyPort != 0 {
			desiredProxyPort = strconv.Itoa(herdrProxyPort)
		}

		if currentImage == imageRef && existingHerdrSocket == herdrSocket && existingProxyPort == desiredProxyPort {
			// Same image and herdr state - reuse container
			if running {
				// Already running - just exec into it
				fmt.Printf("Attaching to running container %s...\n", containerName)
				if err := dockerClient.Attach(ctx, containerID, command, attachEnv); err != nil {
					return fmt.Errorf("attaching to container: %w", err)
				}
				return nil
			}
			// Same image but stopped - start it
			fmt.Printf("Starting container %s...\n", containerName)
			if err := dockerClient.StartContainer(ctx, containerID); err != nil {
				return fmt.Errorf("starting container: %w", err)
			}
			if err := dockerClient.Attach(ctx, containerID, command, attachEnv); err != nil {
				return fmt.Errorf("attaching to container: %w", err)
			}
			return nil
		}

		// Image or herdr state changed - remove and recreate
		fmt.Printf("Config changed, recreating container...\n")
		if err := dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
			return fmt.Errorf("removing old container: %w", err)
		}
	}

	// Create new container
	fmt.Printf("Creating container %s...\n", containerName)
	containerID, err = dockerClient.CreateContainer(ctx, docker.ContainerConfig{
		ImageRef:            imageRef,
		ContainerName:       containerName,
		WorkDir:             cwd,
		Ports:               cfg.GetAllPorts(),
		Env:                 cfg.Env,
		Command:             command,
		HerdrSocketHostPath: herdrSocket,
		HerdrProxyPort:      herdrProxyPort,
	})
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	// Start container
	fmt.Printf("Starting container...\n")
	if err := dockerClient.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	// Attach to container
	if err := dockerClient.Attach(ctx, containerID, command, attachEnv); err != nil {
		return fmt.Errorf("attaching to container: %w", err)
	}

	return nil
}

// maybeReexecForHerdr re-executes the current rig process with HERDR_AGENT set
// when running inside herdr and the variable is not yet present. herdr detects
// the agent by reading the foreground process's /proc/<pid>/environ, which only
// reflects the environment captured at exec time — so an in-process os.Setenv
// would be invisible to it. Re-exec is the reliable way to expose the variable.
// It returns nil (a no-op) when no re-exec is needed.
func maybeReexecForHerdr(agent string) error {
	if !herdr.NeedsReexec(os.LookupEnv) {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		// Non-fatal: fall back to running without the detection hint.
		return nil
	}
	env := herdr.ReexecEnv(os.Environ(), agent)
	return syscall.Exec(exe, os.Args, env)
}
