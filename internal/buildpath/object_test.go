package buildpath

import "testing"

func TestObjectNamesDisambiguatesDuplicateBasenames(t *testing.T) {
	names := ObjectNames([]string{"src/client/main.cpp", "src/server/main.cpp", "src/util.cpp"})
	if names["src/client/main.cpp"] == names["src/server/main.cpp"] {
		t.Fatal("duplicate source basenames produced the same object name")
	}
	if names["src/util.cpp"] != "util.cpp.o" {
		t.Errorf("unique basename changed: %q", names["src/util.cpp"])
	}
}
