package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecreateReasons(t *testing.T) {
	const (
		idA = "sha256:42931428188de8a16c0b0c534155b837ea816f096acef003066d4d572bec0bf2"
		idB = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	base := ContainerState{
		ImageID:      idA,
		HerdrSocket:  "/home/user/.config/herdr/herdr.sock",
		DockerSocket: "/run/user/1000/podman/podman.sock",
		UsernsMode:   UsernsKeepID,
	}

	tests := []struct {
		name     string
		existing ContainerState
		desired  ContainerState
		expected []string
	}{
		{
			name:     "identical state needs no recreation",
			existing: base,
			desired:  base,
			expected: nil,
		},
		{
			name:     "empty states need no recreation",
			existing: ContainerState{},
			desired:  ContainerState{},
			expected: nil,
		},
		{
			name:     "different image reports short IDs",
			existing: base,
			desired:  withImageID(base, idB),
			expected: []string{"image: 42931428188d -> 0123456789ab"},
		},
		{
			name:     "image IDs differing only after 12 characters still differ",
			existing: withImageID(base, "sha256:42931428188dffff"),
			desired:  base,
			expected: []string{"image: 42931428188d -> 42931428188d"},
		},
		{
			name:     "added herdr socket names the missing side",
			existing: ContainerState{ImageID: idA},
			desired:  ContainerState{ImageID: idA, HerdrSocket: "/tmp/herdr.sock"},
			expected: []string{"herdr socket: (none) -> /tmp/herdr.sock"},
		},
		{
			name:     "removed herdr bridge port",
			existing: ContainerState{HerdrProxyPort: "41234"},
			desired:  ContainerState{},
			expected: []string{"herdr bridge port: 41234 -> (none)"},
		},
		{
			name:     "every field changed is reported in order",
			existing: base,
			desired: ContainerState{
				ImageID:        idB,
				HerdrSocket:    "/other/herdr.sock",
				HerdrProxyPort: "41234",
				DockerSocket:   ContainerDockerSocket,
				UsernsMode:     "",
			},
			expected: []string{
				"image: 42931428188d -> 0123456789ab",
				"herdr socket: /home/user/.config/herdr/herdr.sock -> /other/herdr.sock",
				"herdr bridge port: (none) -> 41234",
				"engine socket: /run/user/1000/podman/podman.sock -> /var/run/docker.sock",
				"user namespace mode: keep-id -> (none)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, RecreateReasons(tt.existing, tt.desired))
		})
	}
}

func withImageID(state ContainerState, id string) ContainerState {
	state.ImageID = id
	return state
}

func TestDisplayValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty is named", "", "(none)"},
		{"long image ID is shortened", "sha256:42931428188de8a16c0b", "42931428188d"},
		{"short image ID loses only its prefix", "sha256:42931428188d", "42931428188d"},
		{"non-ID value is unchanged", "/var/run/docker.sock", "/var/run/docker.sock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, displayValue(tt.input))
		})
	}
}
