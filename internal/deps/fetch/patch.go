package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"

	"github.com/loov/clue/internal/deps"
)

// derivePatched makes target a patched copy of pristine: it copies pristine
// (without .git and the fetch marker) next to target, applies the patches in
// order, writes marker and renames the copy into place. target is replaced
// only once every patch applied, and older or interrupted patched copies of
// pristine are removed.
func derivePatched(ctx context.Context, pristine, target string, patches []deps.Patch, projectDir string, marker []byte) (resultErr error) {
	temporary, err := os.MkdirTemp(filepath.Dir(target), filepath.Base(pristine)+".patching-*")
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, os.RemoveAll(temporary))
		}
	}()
	if err := copySources(ctx, pristine, temporary); err != nil {
		return fmt.Errorf("copy %s: %w", pristine, err)
	}
	for _, patch := range patches {
		if err := applyPatch(temporary, patch.Data); err != nil {
			return fmt.Errorf("patch %s: %w", displayPath(projectDir, patch.Path), err)
		}
	}
	if err := os.WriteFile(filepath.Join(temporary, ".clue-dep"), marker, 0o644); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Dir(pristine))
	if err != nil {
		return err
	}
	base := filepath.Base(pristine)
	for _, entry := range entries {
		path := filepath.Join(filepath.Dir(pristine), entry.Name())
		stale := strings.HasPrefix(entry.Name(), base+".patched-") || strings.HasPrefix(entry.Name(), base+".patching-")
		if stale && path != temporary {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
	}
	return os.Rename(temporary, target)
}

// displayPath shows path relative to the project when it is inside it.
func displayPath(projectDir, path string) string {
	if absolute, err := filepath.Abs(projectDir); err == nil {
		if relative, err := filepath.Rel(absolute, path); err == nil && filepath.IsLocal(relative) {
			return filepath.ToSlash(relative)
		}
	}
	return path
}

// copySources copies the directory tree src to dst, leaving out Git metadata
// and the fetch marker.
func copySources(ctx context.Context, src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if entry.Name() == ".git" || relative == ".clue-dep" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			return copyFile(path, target, info.Mode().Perm())
		}
	})
}

func copyFile(src, dst string, mode fs.FileMode) (resultErr error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, in.Close()) }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}

// applyPatch applies a unified diff to the files in dir. Hunks must match
// the files exactly where they say, as the sources are pinned. Git diffs
// name files without their a/ and b/ prefixes; other unified diffs have
// their first path component removed, as patch -p1 does.
func applyPatch(dir string, data []byte) error {
	files, _, err := gitdiff.Parse(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no file changes found")
	}
	stripFirst := !bytes.HasPrefix(data, []byte("diff --git ")) && !bytes.Contains(data, []byte("\ndiff --git "))
	// The root keeps symlinks in the sources from leading out of dir.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for _, file := range files {
		oldName, newName, err := patchNames(file, stripFirst)
		if err != nil {
			return err
		}
		name := newName
		if name == "" {
			name = oldName
		}
		if err := applyFile(root, file, filepath.FromSlash(oldName), filepath.FromSlash(newName)); err != nil {
			return fmt.Errorf("file %s: %w", name, err)
		}
	}
	return nil
}

// patchNames returns the names a file change reads from and writes to,
// relative to the patched directory; either is empty for a new or deleted file.
func patchNames(file *gitdiff.File, stripFirst bool) (oldName, newName string, err error) {
	clean := func(name string) (string, error) {
		if name == "" {
			return "", nil
		}
		if stripFirst {
			_, rest, ok := strings.Cut(name, "/")
			if !ok {
				return "", fmt.Errorf("file name %q has no directory to strip (the patch should be made with a/ and b/ prefixes)", name)
			}
			name = rest
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return "", fmt.Errorf("file name %q is outside the dependency", name)
		}
		return name, nil
	}
	if !file.IsNew {
		if oldName, err = clean(file.OldName); err != nil {
			return "", "", err
		}
	}
	if !file.IsDelete {
		if newName, err = clean(file.NewName); err != nil {
			return "", "", err
		}
	}
	return oldName, newName, nil
}

func applyFile(root *os.Root, file *gitdiff.File, oldName, newName string) error {
	var source []byte
	mode := fs.FileMode(0o644)
	if oldName != "" {
		info, err := root.Lstat(oldName)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", filepath.ToSlash(oldName))
		}
		mode = info.Mode().Perm()
		if source, err = root.ReadFile(oldName); err != nil {
			return err
		}
	} else if _, err := root.Lstat(newName); err == nil {
		return errors.New("the patch creates it, but it exists")
	}

	var result bytes.Buffer
	if err := gitdiff.Apply(&result, bytes.NewReader(source), file); err != nil {
		var applyErr *gitdiff.ApplyError
		if errors.As(err, &applyErr) && applyErr.Fragment > 0 {
			return fmt.Errorf("hunk %d (line %d of the file): %w", applyErr.Fragment, applyErr.Line, err)
		}
		return err
	}

	if newName == "" {
		return root.Remove(oldName)
	}
	if file.NewMode != 0 {
		mode = file.NewMode.Perm()
	}
	if err := root.MkdirAll(filepath.Dir(newName), 0o755); err != nil {
		return err
	}
	if oldName != "" && !file.IsCopy {
		// Removing it first also applies the mode of the result.
		if err := root.Remove(oldName); err != nil {
			return err
		}
	}
	return root.WriteFile(newName, result.Bytes(), mode)
}
