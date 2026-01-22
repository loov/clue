package graph

// NodeType identifies what kind of build artifact a node represents
type NodeType string

const (
	NodeTypeSource     NodeType = "source"     // .c, .cpp files
	NodeTypeHeader     NodeType = "header"     // .h, .hpp files (for dependency tracking)
	NodeTypeObject     NodeType = "object"     // .o files
	NodeTypeExecutable NodeType = "executable" // final binary
	NodeTypeStatic     NodeType = "static"     // .a static library
	NodeTypeShared     NodeType = "shared"     // .so/.dylib shared library
	NodeTypeCommand    NodeType = "command"    // compile/link command (represents transformation)
)

// Node represents a vertex in the dependency graph
type Node struct {
	// ID is the unique identifier (target name or file path)
	ID string

	// Type indicates what kind of artifact this node represents
	Type NodeType

	// Path is the file system path for this artifact
	Path string

	// Metadata holds target-specific configuration
	Metadata map[string]any
}

// NodeID extracts the unique identifier for graph operations
func NodeID(n Node) string {
	return n.ID
}

// CommandInfo holds information about a build command
type CommandInfo struct {
	// Tool is the executable to run (e.g., "clang++", "ar")
	Tool string

	// Args are the command line arguments
	Args []string

	// Inputs are the source files (by node ID)
	Inputs []string

	// Output is the produced file (by node ID)
	Output string
}

// FileNode represents a file in the build graph with its dependencies
type FileNode struct {
	Node

	// Command is the command that produces this file (nil for source files)
	Command *CommandInfo

	// Dependencies are the file IDs this file depends on
	Dependencies []string
}
