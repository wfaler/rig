package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagMatchesImageName(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected bool
	}{
		{"bare name with tag (Docker)", "rig-rig:e2af40132020", true},
		{"bare name without tag", "rig-rig", true},
		{"docker.io qualified (Podman)", "docker.io/library/rig-rig:e2af40132020", true},
		{"localhost qualified (podman build)", "localhost/rig-rig:e2af40132020", true},
		{"longer name sharing the prefix", "rig-rig-extra:abc", false},
		{"qualified longer name sharing the prefix", "docker.io/library/rig-rigger:abc", false},
		{"other registry is not ours", "ghcr.io/someone/rig-rig:abc", false},
		{"other docker.io namespace is not ours", "docker.io/someone/rig-rig:abc", false},
		{"unrelated image", "postgres:16", false},
		{"empty tag", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tagMatchesImageName(tt.tag, "rig-rig"))
		})
	}
}
