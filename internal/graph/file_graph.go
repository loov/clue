package graph

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FileGraphBuilder constructs file-level dependency graphs
type FileGraphBuilder struct {
	builder *Builder
	nodes   map[string]FileNode
}

// NewFileGraphBuilder creates a new file-level graph builder
func NewFileGraphBuilder() *FileGraphBuilder {
	return &FileGraphBuilder{
		builder: NewBuilder(),
		nodes:   make(map[string]FileNode),
	}
}

// TargetFiles holds the file information for a build target
type TargetFiles struct {
	Name     string
	Type     string // "executable", "static_library", "shared_library"
	Sources  []string
	Headers  []string
	Includes []string
	Defines  []string
	Flags    []string
	Depends  []string // Other target names
}

// BuildFileGraph creates a file-level dependency graph from target information.
// For each source file, it creates: source -> compile_cmd -> object
// Then links all objects to the output via a link command.
func (b *FileGraphBuilder) BuildFileGraph(buildDir string, targets []TargetFiles) (*BuildGraph, error) {
	// Process each target
	for _, target := range targets {
		if err := b.addTargetFiles(buildDir, target); err != nil {
			return nil, err
		}
	}

	// Add cross-target dependencies
	for _, target := range targets {
		for _, dep := range target.Depends {
			// Find the output node for the dependency target
			depOutput := b.outputNodeID(dep)
			targetOutput := b.outputNodeID(target.Name)

			// The target's link command depends on the dependency's output
			linkCmd := b.linkCmdNodeID(target.Name)
			b.builder.AddDependency(linkCmd, depOutput)

			// Also need to track the output dependency for proper ordering
			b.builder.AddDependency(targetOutput, depOutput)
		}
	}

	return b.builder.Build()
}

// addTargetFiles adds all file-level nodes for a single target
func (b *FileGraphBuilder) addTargetFiles(buildDir string, target TargetFiles) error {
	var objectNodes []string

	// Add source and object nodes for each source file
	for _, source := range target.Sources {
		sourceID := b.sourceNodeID(target.Name, source)
		objectID := b.objectNodeID(target.Name, source, buildDir)
		compileID := b.compileCmdNodeID(target.Name, source)

		// Add source node
		sourceNode := Node{
			ID:   sourceID,
			Type: NodeTypeSource,
			Path: source,
			Metadata: map[string]any{
				"target": target.Name,
			},
		}
		if err := b.builder.AddNode(sourceNode); err != nil {
			return err
		}

		// Add compile command node
		cmdNode := Node{
			ID:   compileID,
			Type: NodeTypeCommand,
			Path: "",
			Metadata: map[string]any{
				"tool":     "compiler",
				"input":    source,
				"output":   objectID,
				"includes": target.Includes,
				"defines":  target.Defines,
				"flags":    target.Flags,
			},
		}
		if err := b.builder.AddNode(cmdNode); err != nil {
			return err
		}

		// Add object node
		objectPath := b.objectPath(target.Name, source, buildDir)
		objectNode := Node{
			ID:   objectID,
			Type: NodeTypeObject,
			Path: objectPath,
			Metadata: map[string]any{
				"source": source,
				"target": target.Name,
			},
		}
		if err := b.builder.AddNode(objectNode); err != nil {
			return err
		}

		// Wire: source -> compile_cmd -> object
		b.builder.AddDependency(compileID, sourceID)
		b.builder.AddDependency(objectID, compileID)

		objectNodes = append(objectNodes, objectID)
	}

	// Add link command and output nodes
	linkID := b.linkCmdNodeID(target.Name)
	outputID := b.outputNodeID(target.Name)

	// Add link command node
	linkNode := Node{
		ID:   linkID,
		Type: NodeTypeCommand,
		Path: "",
		Metadata: map[string]any{
			"tool":   b.linkerTool(target.Type),
			"inputs": objectNodes,
			"output": outputID,
			"flags":  target.Flags,
			"type":   target.Type,
		},
	}
	if err := b.builder.AddNode(linkNode); err != nil {
		return err
	}

	// Add output node
	outputNode := Node{
		ID:   outputID,
		Type: b.outputNodeType(target.Type),
		Path: b.outputPath(target.Name, target.Type, buildDir),
		Metadata: map[string]any{
			"target":  target.Name,
			"type":    target.Type,
			"objects": objectNodes,
		},
	}
	if err := b.builder.AddNode(outputNode); err != nil {
		return err
	}

	// Wire: all objects -> link_cmd -> output
	for _, objID := range objectNodes {
		b.builder.AddDependency(linkID, objID)
	}
	b.builder.AddDependency(outputID, linkID)

	return nil
}

// Node ID generators for consistent naming
func (b *FileGraphBuilder) sourceNodeID(target, source string) string {
	return fmt.Sprintf("src:%s:%s", target, source)
}

func (b *FileGraphBuilder) objectNodeID(target, source, buildDir string) string {
	return fmt.Sprintf("obj:%s:%s", target, b.objectPath(target, source, buildDir))
}

func (b *FileGraphBuilder) compileCmdNodeID(target, source string) string {
	return fmt.Sprintf("cmd:compile:%s:%s", target, source)
}

func (b *FileGraphBuilder) linkCmdNodeID(target string) string {
	return fmt.Sprintf("cmd:link:%s", target)
}

func (b *FileGraphBuilder) outputNodeID(target string) string {
	return fmt.Sprintf("out:%s", target)
}

// Path generators
func (b *FileGraphBuilder) objectPath(target, source, buildDir string) string {
	// Replace extension with .o and put in build dir
	base := filepath.Base(source)
	ext := filepath.Ext(base)
	obj := strings.TrimSuffix(base, ext) + ".o"
	return filepath.Join(buildDir, target, obj)
}

func (b *FileGraphBuilder) outputPath(target, targetType, buildDir string) string {
	switch targetType {
	case "executable":
		return filepath.Join(buildDir, target)
	case "static_library":
		return filepath.Join(buildDir, "lib"+target+".a")
	case "shared_library":
		return filepath.Join(buildDir, "lib"+target+".so")
	default:
		return filepath.Join(buildDir, target)
	}
}

func (b *FileGraphBuilder) outputNodeType(targetType string) NodeType {
	switch targetType {
	case "executable":
		return NodeTypeExecutable
	case "static_library":
		return NodeTypeStatic
	case "shared_library":
		return NodeTypeShared
	default:
		return NodeTypeExecutable
	}
}

func (b *FileGraphBuilder) linkerTool(targetType string) string {
	switch targetType {
	case "static_library":
		return "ar"
	default:
		return "linker"
	}
}

// GetNodes returns all registered file nodes
func (b *FileGraphBuilder) GetNodes() map[string]FileNode {
	return b.nodes
}
