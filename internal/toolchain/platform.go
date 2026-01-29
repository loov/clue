package toolchain

import (
	"fmt"
	"runtime"
	"strings"
)

// Platform represents an OS-architecture combination for cross-platform builds.
type Platform struct {
	OS   string // "linux", "darwin"
	Arch string // "amd64", "arm64"
}

// HostPlatform returns the platform this binary was built for using Go runtime constants.
func HostPlatform() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// String returns the Go-style "os-arch" format (e.g., "linux-amd64", "darwin-arm64").
func (p Platform) String() string {
	return fmt.Sprintf("%s-%s", p.OS, p.Arch)
}

// IsCrossCompile checks if this platform differs from the host platform.
func (p Platform) IsCrossCompile() bool {
	host := HostPlatform()
	return p.OS != host.OS || p.Arch != host.Arch
}

// supportedPlatforms defines the platforms supported for building.
var supportedPlatforms = map[string]bool{
	"linux-amd64":  true,
	"linux-arm64":  true,
	"darwin-amd64": true,
	"darwin-arm64": true,
}

// IsSupportedTarget checks if a platform is supported for building.
func IsSupportedTarget(p Platform) bool {
	return supportedPlatforms[p.String()]
}

// SupportedTargetsList returns a list of supported target platforms for error messages.
func SupportedTargetsList() []string {
	return []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"}
}

// ParseTarget parses a target flag in "os-arch" format into a Platform.
// Returns an error if the format is invalid or the target is unsupported.
func ParseTarget(flag string) (Platform, error) {
	parts := strings.Split(flag, "-")
	if len(parts) != 2 {
		return Platform{}, fmt.Errorf("invalid target format: %s (expected: os-arch)", flag)
	}

	p := Platform{
		OS:   parts[0],
		Arch: parts[1],
	}

	if !IsSupportedTarget(p) {
		return Platform{}, fmt.Errorf("unsupported target: %s\nSupported: %s",
			p.String(),
			strings.Join(SupportedTargetsList(), ", "))
	}

	return p, nil
}
