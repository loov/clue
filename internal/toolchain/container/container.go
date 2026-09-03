// Package container runs a C/C++ toolchain inside a Docker image.
package container

import (
	"fmt"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// Toolchain decorates a compiler toolchain with Docker command invocation.
type Toolchain struct {
	base          toolchain.Toolchain
	docker        string
	image         string
	imageID       string
	hostRoot      string
	containerRoot string
	target        toolchain.Platform
	user          string
	run           func(string, ...string) ([]byte, error)
}

// New creates a Docker-backed toolchain rooted at the project directory.
func New(base toolchain.Toolchain, image, projectDir, workDir string, target toolchain.Platform) (*Toolchain, error) {
	if image == "" {
		return nil, fmt.Errorf("docker toolchain image is required")
	}
	if workDir == "" {
		workDir = "/workspace"
	}
	if !path.IsAbs(workDir) {
		return nil, fmt.Errorf("docker toolchain workdir must be an absolute container path")
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker not found: install Docker or remove toolchain.container")
	}
	hostRoot, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve Docker project mount: %w", err)
	}
	containerUser := ""
	if runtime.GOOS != "windows" {
		if current, err := user.Current(); err == nil && current.Uid != "" && current.Gid != "" {
			containerUser = current.Uid + ":" + current.Gid
		}
	}
	return &Toolchain{
		base: base, docker: docker, image: image, hostRoot: hostRoot,
		containerRoot: path.Clean(workDir), target: target, user: containerUser,
	}, nil
}

func (t *Toolchain) CC() string            { return t.base.CC() }
func (t *Toolchain) CXX() string           { return t.base.CXX() }
func (t *Toolchain) AR() string            { return t.base.AR() }
func (t *Toolchain) Name() string          { return t.base.Name() }
func (t *Toolchain) IsCrossCompiler() bool { return t.target != toolchain.HostPlatform() }
func (t *Toolchain) String() string        { return fmt.Sprintf("%s in docker:%s", t.base.Name(), t.image) }
func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
	return t.base.CompilerFlags(config)
}

func (t *Toolchain) LinkerFlags(config toolchain.Config, sysLibs []string) []string {
	return t.base.LinkerFlags(config, sysLibs)
}

func (t *Toolchain) Identity() (toolchain.CompilerIdentity, error) {
	output, err := t.output("run", "--rm", t.image, t.base.CC(), "--version")
	if err != nil {
		return toolchain.CompilerIdentity{}, fmt.Errorf("identify compiler in Docker image %q: %w", t.image, err)
	}
	image := t.imageID
	if image == "" {
		image = t.image
	}
	return toolchain.CompilerIdentity{Path: image + "\x00" + strings.TrimSpace(string(output)), Size: int64(len(output))}, nil
}

// WrapCommand returns a docker invocation for a toolchain command.
func (t *Toolchain) WrapCommand(name string, args []string, workDir string) (string, []string) {
	containerWorkDir := t.containerRoot
	if workDir != "" {
		if absolute, err := filepath.Abs(workDir); err == nil && strings.HasPrefix(absolute, t.hostRoot+string(filepath.Separator)) {
			containerWorkDir = t.containerRoot + filepath.ToSlash(strings.TrimPrefix(absolute, t.hostRoot))
		}
	}
	dockerArgs := []string{"run", "--rm", "-v", t.hostRoot + ":" + t.containerRoot, "-w", containerWorkDir}
	if t.user != "" {
		dockerArgs = append(dockerArgs, "--user", t.user)
	}
	dockerArgs = append(dockerArgs, t.image, name)
	for _, arg := range args {
		mapped := strings.ReplaceAll(arg, t.hostRoot, t.containerRoot)
		if runtime.GOOS == "windows" && mapped != arg {
			mapped = filepath.ToSlash(mapped)
		}
		dockerArgs = append(dockerArgs, mapped)
	}
	return t.docker, dockerArgs
}

// HostTool is the local executable used to launch container commands.
func (t *Toolchain) HostTool() string { return t.docker }

// CacheKey distinguishes compiler images even when the Docker client is unchanged.
func (t *Toolchain) CacheKey() string {
	image := t.imageID
	if image == "" {
		image = t.image
	}
	return image + "\x00" + t.containerRoot + "\x00" + t.base.Name()
}

// Validate checks that Docker can find the configured image locally.
func (t *Toolchain) Validate() error {
	output, err := t.output("image", "inspect", "--format", "{{.Id}}", t.image)
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("docker image %q is unavailable: %s", t.image, message)
		}
		return fmt.Errorf("docker image %q is unavailable: %w", t.image, err)
	}
	t.imageID = strings.TrimSpace(string(output))
	for _, command := range []string{t.base.CC(), t.base.AR()} {
		output, err := t.output("run", "--rm", t.image, command, "--version")
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message != "" {
				return fmt.Errorf("tool %q is unavailable in Docker image %q: %s", command, t.image, message)
			}
			return fmt.Errorf("tool %q is unavailable in Docker image %q: %w", command, t.image, err)
		}
	}
	return nil
}

func (t *Toolchain) output(args ...string) ([]byte, error) {
	if t.run != nil {
		return t.run(t.docker, args...)
	}
	return exec.Command(t.docker, args...).CombinedOutput()
}
