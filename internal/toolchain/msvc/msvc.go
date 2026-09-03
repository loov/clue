package msvc

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// MSVC-specific flag mapping tables

// optimizationFlags maps semantic optimization levels to MSVC flags
var optimizationFlags = map[string]string{
	"none":       "/Od",
	"size":       "/O1",
	"fast":       "/O2",
	"aggressive": "/O2", // MSVC has no /O3
}

// warningFlags maps semantic warning levels to MSVC flags
var warningFlags = map[string][]string{
	"off":      {"/W0"},
	"default":  {"/W3"},
	"strict":   {"/W4"},
	"pedantic": {"/W4", "/permissive-"},
}

// debugFlags maps semantic debug levels to MSVC flags
var debugFlags = map[string]string{
	"none":    "",
	"minimal": "/Z7",
	"full":    "/Zi",
}

// Toolchain implements the toolchain.Toolchain interface for MSVC (cl.exe).
type Toolchain struct {
	installation *Installation
	target       toolchain.Platform
}

// Compile-time interface check
var _ toolchain.Toolchain = (*Toolchain)(nil)

// New creates a new MSVC toolchain with the given installation and target platform.
func New(installation *Installation, target toolchain.Platform) (*Toolchain, error) {
	if installation == nil {
		return nil, fmt.Errorf("nil MSVC installation")
	}
	return &Toolchain{installation: installation, target: target}, nil
}

// FindAndNew discovers MSVC and creates a new toolchain.
// This is a convenience function combining FindMSVC() and New().
func FindAndNew(target toolchain.Platform) (*Toolchain, error) {
	installation, err := FindMSVC()
	if err != nil {
		return nil, err
	}
	return New(installation, target)
}

// CC returns the C compiler path (cl.exe).
func (t *Toolchain) CC() string {
	return t.findTool("cl.exe")
}

// CXX returns the C++ compiler path (cl.exe - MSVC uses same binary).
func (t *Toolchain) CXX() string {
	return t.findTool("cl.exe")
}

// AR returns the archiver path (lib.exe).
func (t *Toolchain) AR() string {
	return t.findTool("lib.exe")
}

// Name returns the toolchain name.
func (t *Toolchain) Name() string {
	return "msvc"
}

// IsCrossCompiler returns true if configured for cross-compilation.
// MSVC cross-compilation is deferred to v0.3.0.
func (t *Toolchain) IsCrossCompiler() bool {
	return false
}

// String returns a descriptive string for build output.
func (t *Toolchain) String() string {
	return "msvc (native)"
}

// CompilerFlags generates MSVC-specific compiler flags.
func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
	var flags []string

	// Always suppress banner per CONTEXT.md
	flags = append(flags, "/nologo")

	// Add optimization flag
	if opt, ok := optimizationFlags[config.Optimize]; ok && opt != "" {
		flags = append(flags, opt)
	}

	// Add warning flags
	if wflags, ok := warningFlags[config.Warnings]; ok {
		flags = append(flags, wflags...)
	}

	// Add warnings-as-errors flag
	if config.WarningsAsErrors {
		flags = append(flags, "/WX")
	}

	// Add debug flag
	if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
		flags = append(flags, dbg)
	}

	// Add CRT flag: /MTd for debug, /MT for release (per CONTEXT.md)
	// Debug = anything other than "none"
	if config.Debug != "" && config.Debug != "none" {
		flags = append(flags, "/MTd")
	} else {
		flags = append(flags, "/MT")
	}

	// Add C++ exception handling (required for most C++ code)
	flags = append(flags, "/EHsc")

	// PIC is not applicable on Windows (all code is position-independent by default)
	// LTO (/GL) deferred to v0.2.0 due to complexity
	// Sanitizers deferred to v0.2.0 (MSVC sanitizer support is limited)
	// Coverage deferred (MSVC coverage is complex)

	// Add /showIncludes for dependency tracking (used by Ninja)
	flags = append(flags, "/showIncludes")

	// Append raw compiler flags
	flags = append(flags, config.RawCompiler...)

	return flags
}

// LinkerFlags generates MSVC-specific linker flags.
func (t *Toolchain) LinkerFlags(config toolchain.Config, sysLibs []string) []string {
	var flags []string

	// Always suppress banner
	flags = append(flags, "/nologo")

	// Add /DEBUG if debug != "none"
	if config.Debug != "" && config.Debug != "none" {
		flags = append(flags, "/DEBUG")
	}

	// Add system libraries (MSVC uses foo.lib format, not -lfoo)
	for _, lib := range sysLibs {
		// If already has .lib extension, use as-is
		if strings.HasSuffix(lib, ".lib") {
			flags = append(flags, lib)
		} else {
			flags = append(flags, lib+".lib")
		}
	}

	// Append raw linker flags
	flags = append(flags, config.RawLinker...)

	return flags
}

// Identity returns the compiler identity for cache keys.
// MSVC version is parsed from cl.exe banner output.
func (t *Toolchain) Identity() (toolchain.CompilerIdentity, error) {
	clPath := t.CC()
	if clPath == "" {
		return toolchain.CompilerIdentity{}, fmt.Errorf("cl.exe not found in MSVC installation")
	}

	// Try to get the full path to cl.exe for identity
	// The captured environment PATH contains the cl.exe location
	fullPath := t.findToolFullPath("cl.exe")
	if fullPath != "" {
		return toolchain.GetCompilerIdentity(fullPath)
	}

	// Fallback: use the version from installation for a synthetic identity
	return toolchain.CompilerIdentity{
		Path:  clPath,
		Mtime: 0, // Unknown
		Size:  0, // Unknown
	}, nil
}

// findTool returns the tool name for use with the MSVC environment.
// The actual path resolution happens via the captured PATH environment.
func (t *Toolchain) findTool(name string) string {
	if fullPath := t.findToolFullPath(name); fullPath != "" {
		return fullPath
	}
	return name
}

// findToolFullPath attempts to find the full path to a tool in the MSVC installation.
func (t *Toolchain) findToolFullPath(name string) string {
	if t.installation == nil || t.installation.Environment == nil {
		return ""
	}

	pathEnv, ok := t.installation.Environment["PATH"]
	if !ok {
		return ""
	}

	// Search PATH directories for the tool
	paths := filepath.SplitList(pathEnv)
	for _, dir := range paths {
		fullPath := filepath.Join(dir, name)
		if fileExists(fullPath) {
			return fullPath
		}
	}

	return ""
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := exec.LookPath(path)
	return err == nil && info != ""
}

// envMapToSlice converts an environment map to a slice of KEY=value strings.
func envMapToSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// GetVersion extracts the MSVC compiler version from cl.exe output.
// The banner format is: "Microsoft (R) C/C++ Optimizing Compiler Version 19.40..."
// This is used for display purposes; Identity() uses file stats for cache keys.
func GetVersion(installation *Installation) (string, error) {
	if installation == nil {
		return "", fmt.Errorf("nil installation")
	}

	// Build command with captured environment
	cmd := exec.Command("cl.exe")
	cmd.Env = envMapToSlice(installation.Environment)

	// cl.exe with no args outputs version banner to stderr
	output, _ := cmd.CombinedOutput()

	// Parse version from banner
	// Format: "Microsoft (R) C/C++ Optimizing Compiler Version 19.40.33811 for x64"
	versionRe := regexp.MustCompile(`Version\s+(\d+\.\d+\.\d+)`)
	matches := versionRe.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		return matches[1], nil
	}

	// Fallback to tools version from installation
	if installation.VCToolsPath != "" {
		return filepath.Base(installation.VCToolsPath), nil
	}

	return "unknown", nil
}

// Environment returns the captured vcvarsall.bat environment variables.
// This is used when spawning cl.exe to ensure the correct paths are set.
func (t *Toolchain) Environment() map[string]string {
	if t.installation == nil {
		return nil
	}
	environment := make(map[string]string, len(t.installation.Environment)+1)
	for key, value := range t.installation.Environment {
		environment[key] = value
	}
	// /showIncludes output is localized. Force the English prefix consumed by
	// direct builds and generated Ninja files.
	environment["VSLANG"] = "1033"
	return environment
}
