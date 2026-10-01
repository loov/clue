package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
	"github.com/loov/layout/format/text"
	"github.com/zeebo/clingy"
)

type graphCommand struct {
	options *options
	format  string
}

func (c *graphCommand) Setup(params clingy.Parameters) {
	c.format = params.Flag("format", "output format (text or svg)", "text").(string)
}

func (c *graphCommand) Execute(context.Context) error {
	o := c.options
	return result(runGraph(os.Stdout, o.dir, o.variant, o.target, c.format))
}

func runGraph(w io.Writer, dir, variant, target, format string) int {
	write, ok := map[string]func(io.Writer, *layout.Graph) error{
		"text": text.Write,
		"svg":  svg.Write,
	}[format]
	if !ok {
		printError(fmt.Errorf("unknown graph format %q (want text or svg)", format))
		return 1
	}

	// The graph is the output, so the configuration summary stays quiet.
	cfg, _, _, err := loadConfig(dir, variant, target, build.VerbosityQuiet)
	if err != nil {
		printError(err)
		return 1
	}

	graph := targetGraph(cfg)
	if format == "text" {
		text.Prepare(graph)
	}
	if err := layout.Hierarchical(graph); err != nil {
		printError(fmt.Errorf("lay out graph: %w", err))
		return 1
	}
	if err := write(w, graph); err != nil {
		printError(err)
		return 1
	}
	return 0
}

// targetGraph draws an edge from every target to each of its dependencies;
// external dependencies are dashed.
func targetGraph(cfg *config.Config) *layout.Graph {
	graph := layout.NewDigraph()
	for _, name := range slices.Sorted(maps.Keys(cfg.Targets)) {
		graph.Node(name)
		for _, dependency := range cfg.Targets[name].Depends {
			if _, ok := cfg.Targets[dependency]; !ok {
				graph.Node(dependency).LineStyle = layout.Dashed
			}
			graph.Edge(name, dependency)
		}
	}
	return graph
}
