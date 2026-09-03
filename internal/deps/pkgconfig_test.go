package deps

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

func TestParsePkgConfigUsage(t *testing.T) {
	usage, err := parsePkgConfigUsage(
		`-I/opt/sdk/include -I '/path with spaces/include' -DSDK=1 -pthread`,
		`-L/opt/sdk/lib -lsdk -Wl,-rpath,/opt/sdk/lib`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(usage.Includes, []string{"/opt/sdk/include", "/path with spaces/include"}) ||
		!reflect.DeepEqual(usage.Defines, []string{"SDK=1"}) ||
		!reflect.DeepEqual(usage.CompilerFlags, []string{"-pthread"}) ||
		!reflect.DeepEqual(usage.LinkerFlags, []string{"-L/opt/sdk/lib", "-lsdk", "-Wl,-rpath,/opt/sdk/lib"}) {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestResolvePkgConfigWithRunner(t *testing.T) {
	pkg := NewPkgConfigDependency("sdk", "clue-sdk", false)
	var calls [][]string
	usage, err := pkg.ResolveWithRunner(t.Context(), func(_ context.Context, name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		if slices.Contains(args, "--cflags") {
			return "-I/container/include -DSDK", nil
		}
		return "-L/container/lib -lsdk", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !reflect.DeepEqual(usage.Includes, []string{"/container/include"}) {
		t.Fatalf("calls=%v usage=%+v", calls, usage)
	}
}
