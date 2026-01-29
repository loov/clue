package build

import "github.com/loov/clue/internal/toolchain"

// Platform represents an OS-architecture combination for cross-platform builds.
type Platform = toolchain.Platform

// HostPlatform returns the platform this binary was built for using Go runtime constants.
var HostPlatform = toolchain.HostPlatform

// IsSupportedTarget checks if a platform is supported for building.
var IsSupportedTarget = toolchain.IsSupportedTarget

// SupportedTargetsList returns a list of supported target platforms for error messages.
var SupportedTargetsList = toolchain.SupportedTargetsList

// ParseTarget parses a target flag in "os-arch" format into a Platform.
// Returns an error if the format is invalid or the target is unsupported.
var ParseTarget = toolchain.ParseTarget
