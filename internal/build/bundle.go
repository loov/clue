package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
)

// finishBundle writes a macOS bundle's Info.plist and PkgInfo around the
// linked module and signs it. It reruns when the module was relinked or the
// Info.plist or signing identity changed.
func (b *Builder) finishBundle(ctx context.Context, opts Options, target config.Target, linkFingerprint []byte) error {
	bundle := plan.BundleLayout(target, opts.BuildDir, opts.Variant, b.target)
	if bundle.InfoPlist == "" {
		return nil // only macOS has a bundle directory
	}
	var plist []byte
	if bundle.Source != "" {
		var err error
		if plist, err = os.ReadFile(bundle.Source); err != nil {
			return fmt.Errorf("bundle %q: %w", target.Name, err)
		}
	} else {
		generated, err := plan.BundleInfoPlist(target, opts.Config.Version)
		if err != nil {
			return err
		}
		plist = []byte(generated)
	}
	fingerprint := fmt.Appendf(nil, "%s\n%s\n%s", linkFingerprint, plist, bundle.Sign)
	if previous, err := os.ReadFile(bundle.Stamp); err == nil && bytes.Equal(previous, fingerprint) && !opts.ForceRebuild {
		if _, err := os.Stat(bundle.Binary); err == nil {
			return nil
		}
	}
	if err := os.RemoveAll(bundle.Dir); err != nil {
		return fmt.Errorf("bundle %q: %w", target.Name, err)
	}
	if err := os.MkdirAll(filepath.Dir(bundle.Binary), 0o755); err != nil {
		return fmt.Errorf("bundle %q: %w", target.Name, err)
	}
	if err := copyFile(bundle.Module, bundle.Binary); err != nil {
		return fmt.Errorf("bundle %q: %w", target.Name, err)
	}
	if err := os.WriteFile(bundle.InfoPlist, plist, 0o644); err != nil {
		return fmt.Errorf("bundle %q: %w", target.Name, err)
	}
	if err := os.WriteFile(bundle.PkgInfo, []byte("BNDL????"), 0o644); err != nil {
		return fmt.Errorf("bundle %q: %w", target.Name, err)
	}
	if bundle.Sign != "" {
		// codesign runs on the host, also with a container toolchain; its
		// output ("replacing existing signature") is shown only on failure.
		signer := newExecutor(executorConfig{Verbose: opts.Verbosity == VerbosityVerbose})
		result, err := signer.RunCommand(ctx, "codesign", "--force", "--sign", bundle.Sign, bundle.Dir)
		if err != nil {
			output := ""
			if result != nil {
				output = strings.TrimSpace(result.Stderr + result.Stdout)
			}
			return fmt.Errorf("signing bundle %q: %w: %s", target.Name, err, output)
		}
	}
	if err := os.MkdirAll(filepath.Dir(bundle.Stamp), 0o755); err != nil {
		return err
	}
	return os.WriteFile(bundle.Stamp, fingerprint, 0o644)
}

// copyFile copies a file with its permissions.
func copyFile(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, info.Mode().Perm())
}
