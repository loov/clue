package deps

import (
	"reflect"
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
