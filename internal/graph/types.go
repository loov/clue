package graph

// NodeType identifies what kind of build artifact a node represents
type NodeType string

const (
	NodeTypeSource     NodeType = "source"     // .c, .cpp files
	NodeTypeObject     NodeType = "object"     // .o files
	NodeTypeExecutable NodeType = "executable" // final binary
	NodeTypeStatic     NodeType = "static"     // .a static library
	NodeTypeShared     NodeType = "shared"     // .so/.dylib shared library
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
