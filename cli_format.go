package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue/format"
	"github.com/zeebo/clingy"
)

type formatCommand struct {
	options *options
	paths   []string
}

func (c *formatCommand) Setup(params clingy.Parameters) {
	c.paths = params.Arg("path", "CUE file or directory to format (default: .)", clingy.Repeated).([]string)
}

func (c *formatCommand) Execute(ctx context.Context) error {
	paths := c.paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	for _, root := range paths {
		root = filepath.Clean(root)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() {
				name := entry.Name()
				if path != root && (name == "cue.mod" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || !strings.HasSuffix(path, ".cue") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			formatted, err := format.Source(data)
			if err != nil {
				return fmt.Errorf("format %s: %w", path, err)
			}
			if bytes.Equal(data, formatted) {
				return nil
			}
			if err := os.WriteFile(path, formatted, 0o666); err != nil {
				return err
			}
			if !c.options.quiet {
				_, err = fmt.Fprintln(clingy.Stdout(ctx), path)
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
