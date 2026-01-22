// Package errors provides a shim for cuelang.org/go/cue/errors.
package errors

// Position represents a source position.
type Position interface {
	Filename() string
	Line() int
	Column() int
}

// Errors returns the errors contained in err.
func Errors(err error) []error {
	if err == nil {
		return nil
	}
	return []error{err}
}

// Details returns detailed error information.
func Details(err error, _ interface{}) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Positions returns positions associated with an error.
func Positions(err error) []Position {
	return nil
}
