package deps

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

func TestParsePkgConfigUsage_SeparatesCompileAndLinkFlags(t *testing.T) {
	usage, err := parsePkgConfigUsage(
		`-I/opt/sdk/include -I '/path with spaces/include' -DSDK=1 -pthread`,
		`-L/opt/sdk/lib -lsdk -Wl,-rpath,/opt/sdk/lib`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(usage.SystemIncludes, []string{"/opt/sdk/include", "/path with spaces/include"}) ||
		!reflect.DeepEqual(usage.Defines, []string{"SDK=1"}) ||
		!reflect.DeepEqual(usage.CompilerFlags, []string{"-pthread"}) ||
		!reflect.DeepEqual(usage.LinkerFlags, []string{"-L/opt/sdk/lib", "-lsdk", "-Wl,-rpath,/opt/sdk/lib"}) {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestResolvePkgConfigWithRunner_QueriesFlagsAndLibraries(t *testing.T) {
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
	if len(calls) != 2 || !reflect.DeepEqual(usage.SystemIncludes, []string{"/container/include"}) {
		t.Fatalf("calls=%v usage=%+v", calls, usage)
	}
}

func TestParsePkgConfigUsage_DropsWarningOptions(t *testing.T) {
	usage, err := parsePkgConfigUsage(`-Wno-float-conversion -Werror -Wp,-D_FORTIFY_SOURCE=2 -Wa,--noexecstack -pthread`, ``)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"-Wp,-D_FORTIFY_SOURCE=2", "-Wa,--noexecstack", "-pthread"}; !reflect.DeepEqual(usage.CompilerFlags, want) {
		t.Errorf("compiler flags = %q, want %q", usage.CompilerFlags, want)
	}
}

func TestParsePkgConfigUsage_DropsDuplicates(t *testing.T) {
	usage, err := parsePkgConfigUsage(
		`-I/v/include -isystem /v/other -I/v/include -DNOMINMAX -DNOMINMAX`,
		`-L/v/lib -lgtest_main -lgtest -L/v/lib -lgmock -lgtest -framework CoreFoundation -lz -framework CoreFoundation`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/v/include", "/v/other"}; !reflect.DeepEqual(usage.SystemIncludes, want) {
		t.Errorf("system includes = %q, want %q", usage.SystemIncludes, want)
	}
	if want := []string{"NOMINMAX"}; !reflect.DeepEqual(usage.Defines, want) {
		t.Errorf("defines = %q, want %q", usage.Defines, want)
	}
	if want := []string{"-L/v/lib", "-lgtest_main", "-lgmock", "-lgtest", "-lz", "-framework", "CoreFoundation"}; !reflect.DeepEqual(usage.LinkerFlags, want) {
		t.Errorf("linker flags = %q, want %q", usage.LinkerFlags, want)
	}
}

func TestUsageMerge_SkipsRepeatsAcrossDependencies(t *testing.T) {
	var usage Usage
	usage.Merge(Usage{SystemIncludes: []string{"/v/include"}, Defines: []string{"NOMINMAX"}, LinkerFlags: []string{"-lcloud", "-lz"}})
	usage.Merge(Usage{SystemIncludes: []string{"/v/include"}, Defines: []string{"NOMINMAX"}, LinkerFlags: []string{"-labsl", "-lz"}})
	if want := []string{"/v/include"}; !reflect.DeepEqual(usage.SystemIncludes, want) {
		t.Errorf("system includes = %q, want %q", usage.SystemIncludes, want)
	}
	if want := []string{"NOMINMAX"}; !reflect.DeepEqual(usage.Defines, want) {
		t.Errorf("defines = %q, want %q", usage.Defines, want)
	}
	if want := []string{"-lcloud", "-labsl", "-lz"}; !reflect.DeepEqual(usage.LinkerFlags, want) {
		t.Errorf("linker flags = %q, want %q", usage.LinkerFlags, want)
	}
}
