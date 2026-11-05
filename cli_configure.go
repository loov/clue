package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/clingy"
)

// configureCommand writes a file from a template, replacing @name@ with
// values. It lets custom targets generate files without a shell.
type configureCommand struct {
	template, output string
	values           []string
}

func (c *configureCommand) Setup(params clingy.Parameters) {
	c.template = params.Arg("template", "template file").(string)
	c.output = params.Arg("output", "file to write").(string)
	c.values = params.Arg("value", "name=text, or name=@file for the file's contents", clingy.Repeated).([]string)
}

func (c *configureCommand) Execute(context.Context) error {
	template, err := os.ReadFile(c.template)
	if err != nil {
		return err
	}
	replacements := make([]string, 0, 2*len(c.values))
	for _, value := range c.values {
		name, text, ok := strings.Cut(value, "=")
		if !ok || name == "" {
			return fmt.Errorf("value %q: want name=text or name=@file", value)
		}
		if file, ok := strings.CutPrefix(text, "@"); ok {
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			text = string(data)
		}
		replacements = append(replacements, "@"+name+"@", text)
	}
	// Replacements apply to the template only, not to the text they insert.
	result := strings.NewReplacer(replacements...).Replace(string(template))
	if err := os.MkdirAll(filepath.Dir(c.output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.output, []byte(result), 0o644)
}
