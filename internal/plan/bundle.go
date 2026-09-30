package plan

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

// Bundle is the layout of a "bundle" target for one variant.
type Bundle struct {
	Dir       string // <dir>/<name>.<extension>; on macOS the bundle directory
	Module    string // where the module is linked
	Binary    string // the module in the bundle; on macOS a signed copy of Module
	InfoPlist string // Contents/Info.plist on macOS, "" elsewhere
	PkgInfo   string // Contents/PkgInfo on macOS, "" elsewhere
	Source    string // Info.plist to copy, "" when it is generated
	Sign      string // codesign identity on macOS, "" for none
	Stamp     string // records the finished bundle, outside the signed directory
}

// BundleLayout returns where a bundle target's files go. On macOS the module
// is linked in the build directory and copied to Contents/MacOS/<name> inside
// <dir>/<name>.<extension>; elsewhere it is linked as <dir>/<name>.<extension>,
// or, with the "vst3" layout on Linux and Windows, as
// <dir>/<name>.<extension>/Contents/<arch>-<os>/<name>.so (.vst3 on Windows).
func BundleLayout(target config.Target, buildDir, variant string, platform toolchain.Platform) Bundle {
	settings := target.Bundle
	name := settings.Name
	if name == "" {
		name = target.Name
	}
	dir := settings.Dir
	if dir == "" {
		dir = "{buildDir}"
	}
	dir = expandVariantPaths([]string{dir}, buildDir, variant)[0]
	bundle := Bundle{
		Dir:   filepath.Join(dir, name+"."+settings.Extension),
		Stamp: filepath.Join(buildDir, variant, target.Name, "bundle.clue-link"),
	}
	if platform.OS != "darwin" {
		bundle.Module, bundle.Binary = bundle.Dir, bundle.Dir
		if settings.Layout == "vst3" && (platform.OS == "linux" || platform.OS == "windows") {
			// The VST3 bundle folder: Contents/<architecture>-<os>/<name><suffix>.
			bundle.Module = filepath.Join(bundle.Dir, "Contents", vst3Architecture(platform), name+vst3Suffix(platform))
			bundle.Binary = bundle.Module
		}
		return bundle
	}
	// Signing changes the file, so the bundle gets a copy of the linked module:
	// the link output stays as the linker wrote it.
	bundle.Module = filepath.Join(buildDir, variant, target.Name, name)
	contents := filepath.Join(bundle.Dir, "Contents")
	bundle.Binary = filepath.Join(contents, "MacOS", name)
	bundle.InfoPlist = filepath.Join(contents, "Info.plist")
	bundle.PkgInfo = filepath.Join(contents, "PkgInfo")
	if settings.InfoPlist != "" {
		bundle.Source = expandVariantPaths([]string{settings.InfoPlist}, buildDir, variant)[0]
	}
	bundle.Sign = settings.Sign
	return bundle
}

// BundleInfoPlist returns the Info.plist generated for a bundle target that
// names no Info.plist file of its own.
func BundleInfoPlist(target config.Target, version string) (string, error) {
	settings := target.Bundle
	if settings.Identifier == "" {
		return "", fmt.Errorf("bundle target %q needs bundle.infoPlist or bundle.identifier", target.Name)
	}
	name := settings.Name
	if name == "" {
		name = target.Name
	}
	if version == "" {
		version = "1.0"
	}
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace
	var plist strings.Builder
	plist.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	for _, entry := range [][2]string{
		{"CFBundleDevelopmentRegion", "English"},
		{"CFBundleExecutable", name},
		{"CFBundleIdentifier", settings.Identifier},
		{"CFBundleInfoDictionaryVersion", "6.0"},
		{"CFBundleName", name},
		{"CFBundlePackageType", "BNDL"},
		{"CFBundleShortVersionString", version},
		{"CFBundleSignature", "????"},
		{"CFBundleVersion", version},
	} {
		fmt.Fprintf(&plist, "\t<key>%s</key>\n\t<string>%s</string>\n", entry[0], escape(entry[1]))
	}
	plist.WriteString("</dict>\n</plist>\n")
	return plist.String(), nil
}

// vst3Architecture names a platform's folder in a VST3 bundle.
func vst3Architecture(platform toolchain.Platform) string {
	switch {
	case platform.OS == "windows" && platform.Arch == "amd64":
		return "x86_64-win"
	case platform.OS == "windows" && platform.Arch == "arm64":
		return "arm64-win"
	case platform.Arch == "amd64":
		return "x86_64-" + platform.OS
	case platform.Arch == "arm64":
		return "aarch64-" + platform.OS
	}
	return platform.Arch + "-" + platform.OS
}

func vst3Suffix(platform toolchain.Platform) string {
	if platform.OS == "windows" {
		return ".vst3"
	}
	return ".so"
}
