// Package container runs a C/C++ toolchain inside a container image.
package container

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// Config describes the container environment for a toolchain.
type Config struct {
	Runtime    string
	Image      string
	ProjectDir string
	WorkDir    string
}

// Toolchain decorates a compiler toolchain with container command invocation.
type Toolchain struct {
	base          toolchain.Toolchain
	runtimePath   string
	image         string
	imageID       string
	hostRoot      string
	containerRoot string
	target        toolchain.Platform
	user          string
	run           func(string, ...string) ([]byte, error)
}

// New creates a container-backed toolchain rooted at the project directory.
func New(base toolchain.Toolchain, config Config, target toolchain.Platform) (*Toolchain, error) {
	if config.Image == "" {
		return nil, fmt.Errorf("container toolchain image is required")
	}
	if config.WorkDir == "" {
		config.WorkDir = "/workspace"
	}
	if !path.IsAbs(config.WorkDir) {
		return nil, fmt.Errorf("container toolchain workdir must be an absolute container path")
	}
	runtimePath, err := findRuntime(config.Runtime)
	if err != nil {
		return nil, err
	}
	hostRoot, err := filepath.Abs(config.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve container project mount: %w", err)
	}
	containerUser := ""
	if runtime.GOOS != "windows" {
		if current, err := user.Current(); err == nil && current.Uid != "" && current.Gid != "" {
			containerUser = current.Uid + ":" + current.Gid
		}
	}
	return &Toolchain{
		base: base, runtimePath: runtimePath, image: config.Image, hostRoot: hostRoot,
		containerRoot: path.Clean(config.WorkDir), target: target, user: containerUser,
	}, nil
}

func findRuntime(name string) (string, error) {
	if name != "" {
		path, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("container runtime %q not found: %w", name, err)
		}
		return path, nil
	}
	for _, candidate := range []string{"docker", "podman", "container", "nerdctl"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("container runtime not found: install Docker, Podman, Apple container, or nerdctl, or set toolchain.container.runtime")
}

func (t *Toolchain) CC() string            { return t.base.CC() }
func (t *Toolchain) CXX() string           { return t.base.CXX() }
func (t *Toolchain) AR() string            { return t.base.AR() }
func (t *Toolchain) Name() string          { return t.base.Name() }
func (t *Toolchain) IsCrossCompiler() bool { return t.target != toolchain.HostPlatform() }
func (t *Toolchain) String() string {
	return fmt.Sprintf("%s in %s:%s", t.base.Name(), filepath.Base(t.runtimePath), t.image)
}

func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
	return t.base.CompilerFlags(config)
}

func (t *Toolchain) LinkerFlags(config toolchain.Config, sysLibs []string) []string {
	return t.base.LinkerFlags(config, sysLibs)
}

func (t *Toolchain) Identity() (toolchain.CompilerIdentity, error) {
	output, err := t.output("run", "--rm", t.image, t.base.CC(), "--version")
	if err != nil {
		return toolchain.CompilerIdentity{}, fmt.Errorf("identify compiler in container image %q: %w", t.image, err)
	}
	image := t.imageID
	if image == "" {
		image = t.image
	}
	return toolchain.CompilerIdentity{Path: image + "\x00" + strings.TrimSpace(string(output)), Size: int64(len(output))}, nil
}

// WrapCommand returns a container runtime invocation for a toolchain command.
func (t *Toolchain) WrapCommand(name string, args []string, workDir string) (string, []string) {
	containerWorkDir := t.containerRoot
	if workDir != "" {
		if absolute, err := filepath.Abs(workDir); err == nil && strings.HasPrefix(absolute, t.hostRoot+string(filepath.Separator)) {
			containerWorkDir = t.containerRoot + filepath.ToSlash(strings.TrimPrefix(absolute, t.hostRoot))
		}
	}
	runtimeArgs := []string{"run", "--rm", "-v", t.hostRoot + ":" + t.containerRoot, "-w", containerWorkDir}
	if t.user != "" {
		runtimeArgs = append(runtimeArgs, "--user", t.user)
	}
	runtimeArgs = append(runtimeArgs, t.image, name)
	for _, arg := range args {
		mapped := strings.ReplaceAll(arg, t.hostRoot, t.containerRoot)
		if runtime.GOOS == "windows" && mapped != arg {
			mapped = filepath.ToSlash(mapped)
		}
		runtimeArgs = append(runtimeArgs, mapped)
	}
	return t.runtimePath, runtimeArgs
}

// HostTool is the local executable used to launch container commands.
func (t *Toolchain) HostTool() string { return t.runtimePath }

// CacheKey distinguishes compiler images even when the container runtime is unchanged.
func (t *Toolchain) CacheKey() string {
	image := t.imageID
	if image == "" {
		image = t.image
	}
	return t.runtimePath + "\x00" + image + "\x00" + t.containerRoot + "\x00" + t.base.Name()
}

// Validate checks that the runtime can find the configured image locally.
func (t *Toolchain) Validate() error {
	output, err := t.output("image", "inspect", t.image)
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("container image %q is unavailable to %s: %s", t.image, filepath.Base(t.runtimePath), message)
		}
		return fmt.Errorf("container image %q is unavailable to %s: %w", t.image, filepath.Base(t.runtimePath), err)
	}
	t.imageID = fmt.Sprintf("%x", sha256.Sum256(output))
	for _, command := range []string{t.base.CC(), t.base.AR()} {
		output, err := t.output("run", "--rm", t.image, command, "--version")
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message != "" {
				return fmt.Errorf("tool %q is unavailable in container image %q: %s", command, t.image, message)
			}
			return fmt.Errorf("tool %q is unavailable in container image %q: %w", command, t.image, err)
		}
	}
	return nil
}

func (t *Toolchain) output(args ...string) ([]byte, error) {
	if t.run != nil {
		return t.run(t.runtimePath, args...)
	}
	return exec.Command(t.runtimePath, args...).CombinedOutput()
}
