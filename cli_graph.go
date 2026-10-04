package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

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
	c.format = params.Flag("format", "output format (text, svg, dot or tgf)", "text").(string)
}

func (c *graphCommand) Execute(context.Context) error {
	o := c.options
	return result(runGraph(os.Stdout, o.dir, o.variant, o.target, c.format))
}

func runGraph(w io.Writer, dir, variant, target, format string) int {
	var write func(io.Writer, *layout.Graph) error
	switch format {
	case "text":
		write = layoutWriter(text.Write, layout.Options{ForText: true})
	case "svg":
		write = layoutWriter(svg.Write, layout.Options{})
	case "dot":
		// DOT and TGF describe only the graph; the program reading them lays it out.
		write = writeDOT
	case "tgf":
		write = writeTGF
	default:
		printError(fmt.Errorf("unknown graph format %q (want text, svg, dot or tgf)", format))
		return 1
	}

	// The graph is the output, so the configuration summary stays quiet.
	cfg, _, _, err := loadConfig(dir, variant, target, build.VerbosityQuiet)
	if err != nil {
		printError(err)
		return 1
	}

	if err := write(w, targetGraph(cfg)); err != nil {
		printError(err)
		return 1
	}
	return 0
}

// layoutWriter lays out the graph hierarchically before writing it.
func layoutWriter(write func(io.Writer, *layout.Layout) error, opts layout.Options) func(io.Writer, *layout.Graph) error {
	return func(w io.Writer, graph *layout.Graph) error {
		l, err := layout.Hierarchical(graph, opts)
		if err != nil {
			return fmt.Errorf("lay out graph: %w", err)
		}
		return write(w, l)
	}
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

// writeDOT writes the graph for Graphviz, as in "clue graph -format dot | dot -Tpng".
func writeDOT(w io.Writer, graph *layout.Graph) error {
	var b strings.Builder
	b.WriteString("digraph {\n")
	for _, node := range graph.Nodes {
		if node.LineStyle == layout.Dashed {
			fmt.Fprintf(&b, "\t%q [style=dashed];\n", node.ID)
		} else {
			fmt.Fprintf(&b, "\t%q;\n", node.ID)
		}
	}
	for _, edge := range graph.Edges {
		fmt.Fprintf(&b, "\t%q -> %q;\n", edge.From.ID, edge.To.ID)
	}
	b.WriteString("}\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// writeTGF writes the graph in the Trivial Graph Format: a line with the
// number and name of each node, "#", and a line with the numbers of the ends
// of each edge.
func writeTGF(w io.Writer, graph *layout.Graph) error {
	var b strings.Builder
	number := make(map[*layout.Node]int, len(graph.Nodes))
	for i, node := range graph.Nodes {
		number[node] = i + 1
		fmt.Fprintf(&b, "%d %s\n", i+1, node.ID)
	}
	b.WriteString("#\n")
	for _, edge := range graph.Edges {
		fmt.Fprintf(&b, "%d %d\n", number[edge.From], number[edge.To])
	}
	_, err := io.WriteString(w, b.String())
	return err
}
