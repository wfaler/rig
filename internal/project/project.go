package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ConfigFileName is the name of the configuration file
	ConfigFileName = ".assistant.yml"

	// HashLength is the number of characters to use from the SHA256 hash for image tags
	HashLength = 12
)

// GetProjectName returns the directory name for use in image/container naming
func GetProjectName(dir string) string {
	return filepath.Base(dir)
}

// ConfigPath returns the full path to the config file in the given directory
func ConfigPath(dir string) string {
	return filepath.Join(dir, ConfigFileName)
}

// ConfigExists checks if .assistant.yml exists in the directory
func ConfigExists(dir string) bool {
	_, err := os.Stat(ConfigPath(dir))
	return err == nil
}

// ComputeConfigHash generates a truncated SHA256 hash of the config file content
func ComputeConfigHash(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("reading config for hash: %w", err)
	}

	return ComputeHash(data), nil
}

// ComputeHash generates a truncated SHA256 hash from bytes
func ComputeHash(data []byte) string {
	hash := sha256.Sum256(data)
	fullHash := hex.EncodeToString(hash[:])
	return fullHash[:HashLength]
}

// ImageRef returns the full image reference (name:tag) for a project
func ImageRef(projectName, configHash string) string {
	return fmt.Sprintf("devbox-%s:%s", projectName, configHash)
}

// ImageName returns just the image name without tag
func ImageName(projectName string) string {
	return fmt.Sprintf("devbox-%s", projectName)
}

// ContainerName returns the container name for a project
func ContainerName(projectName string) string {
	return fmt.Sprintf("devbox-%s", projectName)
}

// GetCurrentDirectory returns the current working directory
func GetCurrentDirectory() (string, error) {
	return os.Getwd()
}
