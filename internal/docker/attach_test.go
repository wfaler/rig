package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvMapToSlice(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
		want []string
	}{
		{"nil map", nil, nil},
		{"empty map", map[string]string{}, nil},
		{
			name: "sorted deterministic output",
			in:   map[string]string{"HERDR_ENV": "1", "HERDR_AGENT": "claude", "HERDR_PANE_ID": "p1"},
			want: []string{"HERDR_AGENT=claude", "HERDR_ENV=1", "HERDR_PANE_ID=p1"},
		},
		{
			name: "value with equals sign preserved",
			in:   map[string]string{"K": "a=b"},
			want: []string{"K=a=b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, envMapToSlice(tt.in))
		})
	}
}
