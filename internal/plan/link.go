package plan

import (
	"path/filepath"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// LinkOptions holds options for linking an executable.
type LinkOptions struct {
	Objects  []string        // Object files to link
	Output   string          // Output executable path
	SysLibs  []string        // System libraries (pthread, m, dl)
	LibPaths []string        // Library search paths (-L)
	Libs     []string        // Additional libraries to link
	Flags    toolchain.Flags // For raw linker flags and debug info
	UseCXX   bool            // Use the C++ driver when the link graph contains C++
}

// SharedLibraryOptions holds options for linking a shared library.
type SharedLibraryOptions struct {
	Objects  []string        // Object files to link
	Output   string          // Output .so/.dylib path
	SysLibs  []string        // System libraries (pthread, m, dl)
	LibPaths []string        // Library search paths (-L)
	Libs     []string        // Additional libraries to link
	Flags    toolchain.Flags // For raw linker flags and debug info
	UseCXX   bool            // Use the C++ driver when the link graph contains C++
	Bundle   bool            // a loadable module: -bundle on macOS, not linkable by others
}

// ArchiveOptions holds options for creating a static library.
type ArchiveOptions struct {
	Objects []string // Object files to archive
	Output  string   // Output static library path (e.g., libfoo.a)
}

// Link returns an executable linker invocation.
func Link(tc toolchain.Toolchain, platform toolchain.Platform, opts LinkOptions) Invocation {
	if tc.Name() == "msvc" {
		args := append([]string(nil), tc.LinkerFlags(opts.Flags, nil)...)
		args = append(args, opts.Objects...)
		args = append(args, "/OUT:"+opts.Output)
		args = append(args, msvcLibraries(tc, platform, opts.LibPaths, opts.Libs, opts.SysLibs)...)
		return Invocation{Tool: msvcTool(tc, "link.exe"), Arguments: args}
	}
	args := append([]string(nil), opts.Objects...)
	args = append(args, "-o", opts.Output)
	args = append(args, gnuLibraries(tc, platform, opts.LibPaths, opts.Libs, opts.SysLibs)...)
	args = append(args, tc.LinkerFlags(opts.Flags, nil)...)
	return Invocation{Tool: linkDriver(tc, opts.UseCXX), Arguments: args}
}

// LinkShared returns a shared-library linker invocation.
func LinkShared(tc toolchain.Toolchain, platform toolchain.Platform, opts SharedLibraryOptions) Invocation {
	importLibrary := ""
	if platform.OS == "windows" {
		importLibrary = strings.TrimSuffix(opts.Output, filepath.Ext(opts.Output)) + ".lib"
	}
	if tc.Name() == "msvc" {
		args := append([]string(nil), tc.LinkerFlags(opts.Flags, nil)...)
		args = append(args, "/DLL")
		args = append(args, opts.Objects...)
		args = append(args, "/OUT:"+opts.Output)
		if importLibrary != "" {
			args = append(args, "/IMPLIB:"+importLibrary)
		}
		args = append(args, msvcLibraries(tc, platform, opts.LibPaths, opts.Libs, opts.SysLibs)...)
		return Invocation{Tool: msvcTool(tc, "link.exe"), Arguments: args, ImportLibrary: importLibrary}
	}
	args := []string{"-shared"}
	switch {
	case platform.IsWASI():
		// WebAssembly has no shared libraries: a module whose exports a host
		// calls, without a main, is a WASI reactor.
		args = []string{"-mexec-model=reactor"}
	case opts.Bundle && platform.OS == "darwin":
		args = []string{"-bundle"}
	}
	args = append(args, opts.Objects...)
	args = append(args, "-o", opts.Output)
	if importLibrary != "" {
		compiler := strings.ToLower(tc.CC())
		if tc.Name() == "gcc" || strings.Contains(compiler, "mingw") || strings.Contains(compiler, "w64") {
			args = append(args, "-Wl,--out-implib,"+importLibrary)
		} else {
			args = append(args, "-Wl,-implib:"+importLibrary)
		}
	}
	libName := filepath.Base(opts.Output)
	switch platform.OS {
	case "darwin":
		if !opts.Bundle {
			args = append(args, "-install_name", "@rpath/"+libName)
		}
	case "linux":
		args = append(args, "-Wl,-soname,"+libName)
	}
	args = append(args, gnuLibraries(tc, platform, opts.LibPaths, opts.Libs, opts.SysLibs)...)
	args = append(args, tc.LinkerFlags(opts.Flags, nil)...)
	return Invocation{Tool: linkDriver(tc, opts.UseCXX), Arguments: args, ImportLibrary: importLibrary}
}

// Archive returns a static-library archiver invocation.
func Archive(tc toolchain.Toolchain, opts ArchiveOptions) Invocation {
	if tc.Name() == "msvc" {
		args := []string{"/nologo", "/OUT:" + opts.Output}
		return Invocation{Tool: tc.AR(), Arguments: append(args, opts.Objects...)}
	}
	args := []string{"crs", opts.Output}
	return Invocation{Tool: tc.AR(), Arguments: append(args, opts.Objects...)}
}

func gnuLibraries(tc toolchain.Toolchain, platform toolchain.Platform, paths, libraries, system []string) []string {
	var args []string
	for _, path := range paths {
		args = append(args, "-L"+path)
	}
	for _, library := range libraries {
		args = append(args, "-l"+library)
	}
	for _, library := range system {
		if flag := toolchain.SystemLibraryFlag(tc.Name(), platform, library); flag != "" {
			args = append(args, flag)
		}
	}
	return args
}

func msvcLibraries(tc toolchain.Toolchain, platform toolchain.Platform, paths, libraries, system []string) []string {
	var args []string
	for _, path := range paths {
		args = append(args, "/LIBPATH:"+path)
	}
	for _, library := range libraries {
		if !strings.HasSuffix(library, ".lib") {
			library += ".lib"
		}
		args = append(args, library)
	}
	for _, library := range system {
		if flag := toolchain.SystemLibraryFlag(tc.Name(), platform, library); flag != "" {
			args = append(args, flag)
		}
	}
	return args
}

func linkDriver(tc toolchain.Toolchain, useCXX bool) string {
	if useCXX {
		return tc.CXX()
	}
	return tc.CC()
}

func msvcTool(tc toolchain.Toolchain, name string) string {
	compiler := tc.CC()
	if filepath.IsAbs(compiler) {
		return filepath.Join(filepath.Dir(compiler), name)
	}
	return name
}

// WholeArchiveArguments returns linker arguments that link every member of
// the given static libraries, not only the members something references.
func WholeArchiveArguments(tc toolchain.Toolchain, platform toolchain.Platform, archives []string) []string {
	var args []string
	for _, archive := range archives {
		switch {
		case tc.Name() == "msvc":
			args = append(args, "/WHOLEARCHIVE:"+archive)
		case platform.OS == "darwin":
			args = append(args, "-Wl,-force_load,"+archive)
		default:
			args = append(args, "-Wl,--whole-archive", archive, "-Wl,--no-whole-archive")
		}
	}
	return args
}

// ExportArguments returns linker arguments that keep the given C symbols in
// the output even when only static libraries define them, and export only
// them. On ELF platforms the export list is a version script at
// versionScript, which the caller writes with VersionScript.
func ExportArguments(tc toolchain.Toolchain, platform toolchain.Platform, symbols []string, versionScript string) []string {
	var args []string
	for _, symbol := range symbols {
		switch {
		case tc.Name() == "msvc":
			args = append(args, "/INCLUDE:"+symbol, "/EXPORT:"+symbol)
		case platform.OS == "darwin":
			args = append(args, "-Wl,-u,_"+symbol, "-Wl,-exported_symbol,_"+symbol)
		case platform.IsWASI():
			args = append(args, "-Wl,--export="+symbol)
		default:
			args = append(args, "-Wl,--undefined="+symbol)
		}
	}
	if len(symbols) > 0 && tc.Name() != "msvc" && platform.OS != "darwin" && platform.OS != "windows" && !platform.IsWASI() {
		args = append(args, "-Wl,--version-script="+versionScript)
	}
	return args
}

// VersionScript returns the ELF version script that exports only symbols.
func VersionScript(symbols []string) string {
	var script strings.Builder
	script.WriteString("{\n  global:\n")
	for _, symbol := range symbols {
		script.WriteString("    " + symbol + ";\n")
	}
	script.WriteString("  local: *;\n};\n")
	return script.String()
}
