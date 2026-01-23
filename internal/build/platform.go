package build

import (
	"fmt"
	"runtime"
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

// supportedPlatforms defines the platforms supported in Phase 5.
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
