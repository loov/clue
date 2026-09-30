// Package deps defines external dependency configuration and resolution.
package deps

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
)

// Dependency represents a generic external dependency
type Dependency interface {
	Name() string
	Type() string
	CachePath(baseDir string) string
	InlineBuild() *InlineConfig
	BuildTarget() string
	ConfigFile() string
	// Description returns the build description given with the dependency
	// (its defaults and targets), or a value that does not exist.
	Description() cue.Value
	Validate() error
}

// InlineConfig defines build configuration for dependencies without clue.cue
type InlineConfig struct {
	Sources  []string
	Exclude  []string // globs of sources to leave out
	Headers  []string
	Includes []string
	Defines  []string
	Depends  []string
	Library  string
	Commands [][]string
	Type     string // built, header-only, or prebuilt library type

	CompilerFlags []string // raw flags for compiling the dependency's sources
	LinkerFlags   []string // raw flags for linking a shared dependency
	Warnings      string   // semantic warning level for its sources
}

// PkgConfigDependency represents a system library described by pkg-config.
type PkgConfigDependency struct {
	name    string
	Package string
	Static  bool
}

// NewPkgConfigDependency creates a pkg-config dependency.
func NewPkgConfigDependency(name, pkg string, static bool) *PkgConfigDependency {
	if pkg == "" {
		pkg = name
	}
	return &PkgConfigDependency{name: name, Package: pkg, Static: static}
}

func (p *PkgConfigDependency) Name() string               { return p.name }
func (p *PkgConfigDependency) Type() string               { return "pkg_config" }
func (p *PkgConfigDependency) CachePath(string) string    { return "" }
func (p *PkgConfigDependency) InlineBuild() *InlineConfig { return nil }
func (p *PkgConfigDependency) BuildTarget() string        { return "" }
func (p *PkgConfigDependency) ConfigFile() string         { return "" }
func (p *PkgConfigDependency) Description() cue.Value     { return cue.Value{} }
func (p *PkgConfigDependency) Validate() error {
	if p.name == "" {
		return fmt.Errorf("pkg-config dependency: name is required")
	}
	if p.Package == "" {
		return fmt.Errorf("pkg-config dependency %q: package is required", p.name)
	}
	return nil
}

// GitDependency represents a dependency fetched from a git repository
type GitDependency struct {
	name        string
	Repo        string
	Ref         string
	Submodules  []string // submodule paths to check out; nil checks out all of them
	Patches     []Patch  // applied in order to a copy of the checkout
	TargetName  string
	File        string    // project clue file describing the build, instead of the dependency's clue.cue
	Spec        cue.Value // build description given with the dependency (defaults, targets)
	Loaded      cue.Value // evaluated file description, scoped to the owning configuration
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

// CachePath returns the directory of the dependency's sources: the
// checkout, or its patched copy when the dependency has patches.
func (g *GitDependency) CachePath(baseDir string) string {
	return patchedPath(g.PristinePath(baseDir), g.Patches)
}

// PristinePath returns the directory of the unpatched checkout.
func (g *GitDependency) PristinePath(baseDir string) string {
	sanitized := sanitizeName(g.name)
	return filepath.Join(baseDir, ".deps", "git", fmt.Sprintf("%s-%s", sanitized, pathSafeRef(g.Ref)))
}

func (g *GitDependency) InlineBuild() *InlineConfig { return g.BuildConfig }
func (g *GitDependency) BuildTarget() string        { return g.TargetName }
func (g *GitDependency) ConfigFile() string         { return g.File }
func (g *GitDependency) Description() cue.Value {
	if g.Spec.Exists() {
		return g.Spec
	}
	return g.Loaded
}

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
	Patches     []Patch // applied in order to a copy of the extracted archive
	TargetName  string
	File        string    // project clue file describing the build, instead of the dependency's clue.cue
	Spec        cue.Value // build description given with the dependency (defaults, targets)
	Loaded      cue.Value // evaluated file description, scoped to the owning configuration
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

// CachePath returns the directory of the dependency's sources: the
// extracted archive, or its patched copy when the dependency has patches.
func (t *TarballDependency) CachePath(baseDir string) string {
	return patchedPath(t.PristinePath(baseDir), t.Patches)
}

// PristinePath returns the directory of the unpatched, extracted archive.
func (t *TarballDependency) PristinePath(baseDir string) string {
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
func (t *TarballDependency) ConfigFile() string         { return t.File }
func (t *TarballDependency) Description() cue.Value {
	if t.Spec.Exists() {
		return t.Spec
	}
	return t.Loaded
}

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
	File        string    // project clue file describing the build, instead of the dependency's clue.cue
	Spec        cue.Value // build description given with the dependency (defaults, targets)
	Loaded      cue.Value // evaluated file description, scoped to the owning configuration
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
func (v *VendoredDependency) ConfigFile() string         { return v.File }
func (v *VendoredDependency) Description() cue.Value {
	if v.Spec.Exists() {
		return v.Spec
	}
	return v.Loaded
}

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

// Patch is a unified diff applied to a dependency's fetched sources.
type Patch struct {
	Path   string // absolute path of the patch file
	Data   []byte // its contents, read when the configuration is loaded
	SHA256 string // hex digest of Data
}

// NewPatch reads the patch file at path.
func NewPatch(path string) (Patch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Patch{}, err
	}
	sum := sha256.Sum256(data)
	return Patch{Path: path, Data: data, SHA256: hex.EncodeToString(sum[:])}, nil
}

// Patches returns the patches of a git or tarball dependency.
func Patches(dependency Dependency) []Patch {
	switch d := dependency.(type) {
	case *GitDependency:
		return d.Patches
	case *TarballDependency:
		return d.Patches
	}
	return nil
}

// PatchesHash identifies a list of patches by their contents and order.
func PatchesHash(patches []Patch) string {
	h := sha256.New()
	for _, patch := range patches {
		h.Write([]byte(patch.SHA256 + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// patchedPath names the patched copy of the sources in pristine, so a copy
// with other patches, or none, is never mistaken for it.
func patchedPath(pristine string, patches []Patch) string {
	if len(patches) == 0 {
		return pristine
	}
	return pristine + ".patched-" + PatchesHash(patches)[:12]
}

// Validate checks that the inline config is valid
func (ic *InlineConfig) Validate() error {
	prebuilt := ic.Type == "prebuilt_static" || ic.Type == "prebuilt_shared"
	external := ic.Type == "external_static" || ic.Type == "external_shared"
	if len(ic.Sources) == 0 && ic.Type != "header_only" && !prebuilt && !external {
		return fmt.Errorf("inline build config: at least one source file is required")
	}
	if (prebuilt || external) && ic.Library == "" {
		return fmt.Errorf("inline build config: library path is required")
	}
	if external && len(ic.Commands) == 0 {
		return fmt.Errorf("inline build config: external build commands are required")
	}
	for _, command := range ic.Commands {
		if len(command) == 0 {
			return fmt.Errorf("inline build config: external build command cannot be empty")
		}
	}
	if ic.Type != "" && ic.Type != "static_library" && ic.Type != "shared_library" &&
		ic.Type != "header_only" && !prebuilt && !external {
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

// pathSafeRef turns a git ref into a directory name component. The whole ref
// is kept, so refs sharing a prefix (v3.8.0_build_66, v3.8.0_build_67) get
// separate checkouts; very long refs are shortened with a hash of the ref.
func pathSafeRef(ref string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_").Replace(ref)
	if len(safe) <= 64 {
		return safe
	}
	sum := sha256.Sum256([]byte(ref))
	return safe[:48] + "-" + hex.EncodeToString(sum[:])[:12]
}

// truncate returns the first n characters of s
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// TargetDependency is a library target of another dependency, named
// "<dependency>:<target>". It uses the checkout of that dependency, which is
// fetched and locked under its own name, and builds the named target of the
// dependency's clue file.
type TargetDependency struct {
	Dependency   // the dependency whose checkout this target builds
	name, target string
}

// Name returns "<dependency>:<target>".
func (t *TargetDependency) Name() string { return t.name }

// BuildTarget returns the target of the dependency's clue file.
func (t *TargetDependency) BuildTarget() string { return t.target }

// Parent returns the dependency that owns the checkout.
func (t *TargetDependency) Parent() Dependency { return t.Dependency }

// ArtifactName returns the file-system name of a dependency's library and
// build directory; "a:b" becomes "a.b", which linkers and file systems accept.
func ArtifactName(name string) string {
	return strings.ReplaceAll(name, ":", ".")
}

// AddTargetDependencies adds a TargetDependency for every "<dependency>:<target>"
// that targets or dependencies refer to. The referenced dependency must build
// from a clue file (its own or a project "file"), not an inline build.
func AddTargetDependencies(dependencies map[string]Dependency, referenced []string) error {
	pending := append([]string(nil), referenced...)
	for _, dependency := range dependencies {
		pending = append(pending, DeclaredDepends(dependency)...)
	}
	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		parentName, target, ok := strings.Cut(name, ":")
		if !ok || dependencies[name] != nil {
			continue
		}
		parent, exists := dependencies[parentName]
		if !exists {
			return fmt.Errorf("%q refers to unknown dependency %q", name, parentName)
		}
		if _, nested := parent.(*TargetDependency); nested || parent.CachePath(".") == "" {
			return fmt.Errorf("%q: dependency %q has no checkout to build targets from", name, parentName)
		}
		if parent.BuildTarget() == target {
			return fmt.Errorf("%q is the library %q itself refers to; depend on %q", name, parentName, parentName)
		}
		if parent.InlineBuild() != nil {
			return fmt.Errorf("%q: dependency %q has an inline build; describe it with a clue file to select targets", name, parentName)
		}
		dependency := &TargetDependency{Dependency: parent, name: name, target: target}
		dependencies[name] = dependency
		pending = append(pending, DeclaredDepends(dependency)...)
	}
	return nil
}
