package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// ImageExists checks if an image with the given ref exists locally
func (c *Client) ImageExists(ctx context.Context, imageRef string) (bool, error) {
	_, _, err := c.cli.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspecting image: %w", err)
	}
	return true, nil
}

// BuildImage builds a Docker image from a Dockerfile string
func (c *Client) BuildImage(ctx context.Context, dockerfile string, imageRef string) error {
	// Create tar archive with Dockerfile in memory
	tarBuf, err := createDockerfileTar(dockerfile)
	if err != nil {
		return fmt.Errorf("creating build context: %w", err)
	}

	resp, err := c.cli.ImageBuild(ctx, tarBuf, types.ImageBuildOptions{
		Tags:        []string{imageRef},
		Dockerfile:  "Dockerfile",
		Remove:      true, // Remove intermediate containers
		ForceRemove: true,
		NoCache:     false,
	})
	if err != nil {
		return fmt.Errorf("starting image build: %w", err)
	}
	defer resp.Body.Close()

	// Stream build output to stdout
	if err := streamBuildOutput(resp.Body); err != nil {
		return fmt.Errorf("streaming build output: %w", err)
	}

	return nil
}

// createDockerfileTar creates an in-memory tar archive containing the Dockerfile
func createDockerfileTar(dockerfile string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	header := &tar.Header{
		Name: "Dockerfile",
		Mode: 0644,
		Size: int64(len(dockerfile)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("writing tar header: %w", err)
	}
	if _, err := tw.Write([]byte(dockerfile)); err != nil {
		return nil, fmt.Errorf("writing dockerfile to tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}

	return &buf, nil
}

// buildMessage represents a message from the Docker build output
type buildMessage struct {
	Stream      string `json:"stream"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

// streamBuildOutput reads and displays Docker build output
func streamBuildOutput(reader io.Reader) error {
	decoder := json.NewDecoder(reader)

	for {
		var msg buildMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decoding build message: %w", err)
		}

		if msg.Error != "" {
			return fmt.Errorf("build error: %s", msg.Error)
		}

		if msg.Stream != "" {
			fmt.Fprint(os.Stdout, msg.Stream)
		}
	}

	return nil
}

// RemoveImagesByName removes all images matching the given name (any tag)
func (c *Client) RemoveImagesByName(ctx context.Context, imageName string) error {
	images, err := c.cli.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("listing images: %w", err)
	}

	var removed int
	for _, img := range images {
		for _, tag := range img.RepoTags {
			// Check if the image name matches (before the :tag)
			if strings.HasPrefix(tag, imageName+":") || tag == imageName {
				fmt.Printf("Removing image %s...\n", tag)
				_, err := c.cli.ImageRemove(ctx, img.ID, image.RemoveOptions{Force: true, PruneChildren: true})
				if err != nil {
					fmt.Printf("Warning: could not remove %s: %v\n", tag, err)
				} else {
					removed++
				}
				break // Only need to remove once per image ID
			}
		}
	}

	if removed == 0 {
		return fmt.Errorf("no images found matching %s", imageName)
	}

	return nil
}
