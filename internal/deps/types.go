package deps

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Dependency represents a generic external dependency
type Dependency interface {
	Name() string
	Type() string
	CachePath(baseDir string) string
	InlineBuild() *InlineConfig
	BuildTarget() string
	Validate() error
}

// InlineConfig defines build configuration for dependencies without clue.cue
type InlineConfig struct {
	Sources  []string
	Headers  []string
	Includes []string
	Defines  []string
	Depends  []string
	Type     string // "static_library", "shared_library", or "header_only"
}

// GitDependency represents a dependency fetched from a git repository
type GitDependency struct {
	name        string
	Repo        string
	Ref         string
	TargetName  string
	BuildConfig *InlineConfig
}

// NewGitDependency creates a new git dependency
func NewGitDependency(name, repo, ref string, buildConfig *InlineConfig) *GitDependency {
	if ref == "" {
		ref = "main"
	}
	return &GitDependency{
		name:        name,
		Repo:        repo,
		Ref:         ref,
		BuildConfig: buildConfig,
	}
}

// Name returns the dependency name
func (g *GitDependency) Name() string {
	return g.name
}

// Type returns "git"
func (g *GitDependency) Type() string {
	return "git"
}

// CachePath returns the cache directory path for this dependency
func (g *GitDependency) CachePath(baseDir string) string {
	sanitized := sanitizeName(g.name)
	shortRef := strings.NewReplacer("/", "_", "\\", "_").Replace(truncate(g.Ref, 12))
	return filepath.Join(baseDir, ".deps", "git", fmt.Sprintf("%s-%s", sanitized, shortRef))
}

func (g *GitDependency) InlineBuild() *InlineConfig { return g.BuildConfig }
func (g *GitDependency) BuildTarget() string        { return g.TargetName }

// Validate checks that required fields are set
func (g *GitDependency) Validate() error {
	if g.name == "" {
		return fmt.Errorf("git dependency: name is required")
	}
	if g.Repo == "" {
		return fmt.Errorf("git dependency %q: repo is required", g.name)
	}
	// Require an authenticated transport.
	if !strings.HasPrefix(g.Repo, "https://") &&
		!strings.HasPrefix(g.Repo, "git@") {
		return fmt.Errorf("git dependency %q: repo must start with https:// or git@", g.name)
	}
	if g.BuildConfig != nil {
		if err := g.BuildConfig.Validate(); err != nil {
			return fmt.Errorf("git dependency %q: %w", g.name, err)
		}
	}
	return nil
}

// TarballDependency represents a dependency fetched from a tarball URL
type TarballDependency struct {
	name        string
	URL         string
	Checksum    string
	StripPrefix string
	TargetName  string
	BuildConfig *InlineConfig
}

// NewTarballDependency creates a new tarball dependency
func NewTarballDependency(name, url, checksum, stripPrefix string, buildConfig *InlineConfig) *TarballDependency {
	return &TarballDependency{
		name:        name,
		URL:         url,
		Checksum:    checksum,
		StripPrefix: stripPrefix,
		BuildConfig: buildConfig,
	}
}

// Name returns the dependency name
func (t *TarballDependency) Name() string {
	return t.name
}

// Type returns "tarball"
func (t *TarballDependency) Type() string {
	return "tarball"
}

// CachePath returns the cache directory path for this dependency
func (t *TarballDependency) CachePath(baseDir string) string {
	sanitized := sanitizeName(t.name)
	var checksumPrefix string
	if t.Checksum != "" {
		checksumPrefix = truncate(t.Checksum, 12)
	} else {
		// Use hash of URL if no checksum provided
		h := sha256.Sum256([]byte(t.URL))
		checksumPrefix = fmt.Sprintf("%x", h[:6])
	}
	return filepath.Join(baseDir, ".deps", "tarball", fmt.Sprintf("%s-%s", sanitized, checksumPrefix))
}

func (t *TarballDependency) InlineBuild() *InlineConfig { return t.BuildConfig }
func (t *TarballDependency) BuildTarget() string        { return t.TargetName }

// Validate checks that required fields are set
func (t *TarballDependency) Validate() error {
	if t.name == "" {
		return fmt.Errorf("tarball dependency: name is required")
	}
	if t.URL == "" {
		return fmt.Errorf("tarball dependency %q: url is required", t.name)
	}
	if !strings.HasPrefix(t.URL, "https://") {
		return fmt.Errorf("tarball dependency %q: url must start with https://", t.name)
	}
	matched, _ := regexp.MatchString("^[a-f0-9]{64}$", t.Checksum)
	if !matched {
		return fmt.Errorf("tarball dependency %q: checksum must be a 64-character SHA256 hex string", t.name)
	}
	if t.BuildConfig != nil {
		if err := t.BuildConfig.Validate(); err != nil {
			return fmt.Errorf("tarball dependency %q: %w", t.name, err)
		}
	}
	return nil
}

// VendoredDependency represents a dependency vendored in the source tree
type VendoredDependency struct {
	name        string
	Path        string
	TargetName  string
	BuildConfig *InlineConfig
}

// NewVendoredDependency creates a new vendored dependency
func NewVendoredDependency(name, path string, buildConfig *InlineConfig) *VendoredDependency {
	return &VendoredDependency{
		name:        name,
		Path:        path,
		BuildConfig: buildConfig,
	}
}

// Name returns the dependency name
func (v *VendoredDependency) Name() string {
	return v.name
}

// Type returns "vendored"
func (v *VendoredDependency) Type() string {
	return "vendored"
}

// CachePath returns the original path (no caching for vendored dependencies)
func (v *VendoredDependency) CachePath(_ string) string {
	return v.Path
}

func (v *VendoredDependency) InlineBuild() *InlineConfig { return v.BuildConfig }
func (v *VendoredDependency) BuildTarget() string        { return v.TargetName }

// Validate checks that required fields are set
func (v *VendoredDependency) Validate() error {
	if v.name == "" {
		return fmt.Errorf("vendored dependency: name is required")
	}
	if v.Path == "" {
		return fmt.Errorf("vendored dependency %q: path is required", v.name)
	}
	if v.BuildConfig != nil {
		if err := v.BuildConfig.Validate(); err != nil {
			return fmt.Errorf("vendored dependency %q: %w", v.name, err)
		}
	}
	return nil
}

// Validate checks that the inline config is valid
func (ic *InlineConfig) Validate() error {
	if len(ic.Sources) == 0 && ic.Type != "header_only" {
		return fmt.Errorf("inline build config: at least one source file is required")
	}
	if ic.Type != "" && ic.Type != "static_library" && ic.Type != "shared_library" && ic.Type != "header_only" {
		return fmt.Errorf("inline build config: unsupported target type %q", ic.Type)
	}
	return nil
}

// Helper functions

// sanitizeName removes characters that are unsafe for filesystem paths
func sanitizeName(name string) string {
	// Replace any non-alphanumeric/underscore/dash with underscore
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	return re.ReplaceAllString(name, "_")
}

// truncate returns the first n characters of s
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
