package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsernsModeFor(t *testing.T) {
	tests := []struct {
		name            string
		components      []string
		securityOptions []string
		expected        string
	}{
		{
			name:            "rootless podman uses keep-id",
			components:      []string{"Podman Engine", "Conmon", "OCI Runtime (crun)"},
			securityOptions: []string{"name=seccomp,profile=default", "name=rootless"},
			expected:        UsernsKeepID,
		},
		{
			name:            "rootful podman uses engine default",
			components:      []string{"Podman Engine"},
			securityOptions: []string{"name=seccomp,profile=default"},
			expected:        "",
		},
		{
			name:            "rootless docker uses engine default",
			components:      []string{"Engine", "containerd", "runc"},
			securityOptions: []string{"name=seccomp,profile=builtin", "name=rootless"},
			expected:        "",
		},
		{
			name:            "rootful docker uses engine default",
			components:      []string{"Engine", "containerd"},
			securityOptions: []string{"name=seccomp,profile=builtin"},
			expected:        "",
		},
		{
			name:     "no engine info uses engine default",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, usernsModeFor(tt.components, tt.securityOptions))
		})
	}
}
