package build

import (
	"testing"

	"github.com/loov/clue/internal/config"
)

func TestBuildTargetInterfaceLibraryNeedsNoTools(t *testing.T) {
	result, err := (&Builder{}).buildTarget(t.Context(), Options{}, config.Target{Name: "headers", Type: "interface_library"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Output != "" {
		t.Fatalf("result = %+v", result)
	}
}
