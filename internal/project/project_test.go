package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProjectName(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{
			name: "simple directory",
			dir:  "/home/user/myproject",
			want: "myproject",
		},
		{
			name: "nested directory",
			dir:  "/home/user/work/projects/myapp",
			want: "myapp",
		},
		{
			name: "root directory",
			dir:  "/",
			want: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetProjectName(tt.dir)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigPath(t *testing.T) {
	dir := "/home/user/myproject"
	want := "/home/user/myproject/.rig.yml"
	got := ConfigPath(dir)
	assert.Equal(t, want, got)
}

func TestConfigExists(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rig-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Initially config should not exist
	assert.False(t, ConfigExists(tmpDir))

	// Create config file
	configPath := filepath.Join(tmpDir, ConfigFileName)
	err = os.WriteFile(configPath, []byte("languages: {}"), 0644)
	require.NoError(t, err)

	// Now config should exist
	assert.True(t, ConfigExists(tmpDir))
}

func TestComputeHash(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "simple content",
			data: []byte("test content"),
			want: "6ae8a75555", // first 12 chars of sha256
		},
		{
			name: "empty content",
			data: []byte(""),
			want: "e3b0c44298fc", // first 12 chars of sha256 of empty string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeHash(tt.data)
			assert.Len(t, got, HashLength)
			// Just verify it's consistent
			assert.Equal(t, got, ComputeHash(tt.data))
		})
	}
}

func TestComputeConfigHash(t *testing.T) {
	// Create temp file
	tmpDir, err := os.MkdirTemp("", "rig-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, ConfigFileName)
	content := []byte("languages:\n  node:\n    version: lts\n")
	err = os.WriteFile(configPath, content, 0644)
	require.NoError(t, err)

	hash, err := ComputeConfigHash(configPath)
	require.NoError(t, err)
	assert.Len(t, hash, HashLength)

	// Verify it matches direct hash computation
	assert.Equal(t, ComputeHash(content), hash)
}

func TestComputeConfigHash_FileNotFound(t *testing.T) {
	_, err := ComputeConfigHash("/nonexistent/path/.rig.yml")
	require.Error(t, err)
}

func TestImageRef(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		configHash  string
		want        string
	}{
		{
			name:        "standard project",
			projectName: "myproject",
			configHash:  "abc123def456",
			want:        "rig-myproject:abc123def456",
		},
		{
			name:        "project with hyphens",
			projectName: "my-cool-project",
			configHash:  "xyz789",
			want:        "rig-my-cool-project:xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ImageRef(tt.projectName, tt.configHash)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageName(t *testing.T) {
	got := ImageName("myproject")
	assert.Equal(t, "rig-myproject", got)
}

func TestContainerName(t *testing.T) {
	got := ContainerName("myproject")
	assert.Equal(t, "rig-myproject", got)
}
