package docker

import (
	"fmt"
	"strings"
)

// ContainerState is the part of a rig container's configuration that is fixed
// at create time, so any difference from the desired state means the container
// must be recreated.
type ContainerState struct {
	ImageID        string // ID of the image the container runs
	HerdrSocket    string // host path of the herdr socket mount, "" if none
	HerdrProxyPort string // herdr TCP bridge port, "" if none
	DockerSocket   string // host path of the engine socket mount, "" if none
	UsernsMode     string // user namespace mode, "" for the engine default
}

// RecreateReasons describes each difference between an existing container's
// state and the desired one, or returns nil when the container can be reused.
func RecreateReasons(existing, desired ContainerState) []string {
	fields := []struct {
		label             string
		existing, desired string
	}{
		{"image", existing.ImageID, desired.ImageID},
		{"herdr socket", existing.HerdrSocket, desired.HerdrSocket},
		{"herdr bridge port", existing.HerdrProxyPort, desired.HerdrProxyPort},
		{"engine socket", existing.DockerSocket, desired.DockerSocket},
		{"user namespace mode", existing.UsernsMode, desired.UsernsMode},
	}

	var reasons []string
	for _, f := range fields {
		if f.existing != f.desired {
			reasons = append(reasons, fmt.Sprintf("%s: %s -> %s", f.label, displayValue(f.existing), displayValue(f.desired)))
		}
	}
	return reasons
}

// displayValue renders a state value for humans, shortening image IDs to the
// 12-character form engines display and naming empty values.
func displayValue(s string) string {
	if s == "" {
		return "(none)"
	}
	id, isImageID := strings.CutPrefix(s, "sha256:")
	if isImageID && len(id) > 12 {
		return id[:12]
	}
	return id
}
