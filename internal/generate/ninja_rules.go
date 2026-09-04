package generate

import (
	"io"
	"strings"

	"github.com/Duncaen/go-ninja"
)

// defaultTarget is a custom Node for the ninja default statement
type defaultTarget struct {
	targets []string
}

func (d defaultTarget) WriteTo(w io.Writer) (int64, error) {
	line := "default " + strings.Join(d.targets, " ") + "\n"
	written, err := io.WriteString(w, line)
	return int64(written), err
}

func (d defaultTarget) RequiredVersion() ninja.Version {
	return ninja.Version(0)
}

func addNinjaRules(file *ninja.File, msvc bool) {
	if msvc {
		*file = append(*file,
			ninja.Rule{
				Name: "cc", Command: `set "VSLANG=1033"&& "$cc" @$object.rsp`, Rspfile: "$object.rsp", RspfileContent: "$args", Deps: ninja.DepsMSVC,
				MSVCDepsPrefix: "Note: including file:", Description: "CC $out",
			},
			ninja.Rule{
				Name: "cxx", Command: `set "VSLANG=1033"&& "$cxx" @$object.rsp`, Rspfile: "$object.rsp", RspfileContent: "$args", Deps: ninja.DepsMSVC,
				MSVCDepsPrefix: "Note: including file:", Description: "CXX $out",
			},
			ninja.Rule{Name: "link", Command: `"$link" $in /OUT:"$out" $ldflags`, Description: "LINK $out"},
			ninja.Rule{Name: "link_shared", Command: `"$link" /DLL $in /OUT:"$out" /IMPLIB:"$implib" $ldflags`, Description: "LINK_SHARED $out"},
			ninja.Rule{Name: "ar", Command: `"$ar" /nologo /OUT:"$out" $in`, Description: "LIB $out"},
			ninja.Rule{Name: "header_unit", Command: `"$cxx" @$out.rsp`, Rspfile: "$out.rsp", RspfileContent: "$huflags", Description: "HEADER_UNIT $out"},
		)
	} else {
		*file = append(*file,
			ninja.Rule{
				Name: "cc", Command: "$cc @$object.rsp", Rspfile: "$object.rsp", RspfileContent: "$args",
				Depfile: "$depfile", Deps: ninja.DepsGCC, Description: "CC $out",
			},
			ninja.Rule{
				Name: "cxx", Command: "$cxx @$object.rsp", Rspfile: "$object.rsp", RspfileContent: "$args",
				Depfile: "$depfile", Deps: ninja.DepsGCC, Description: "CXX $out",
			},
			ninja.Rule{
				Name: "module_partition", Command: "$cxx @$out.rsp", Rspfile: "$out.rsp",
				RspfileContent: "$args", Description: "CXX_MODULE_PARTITION $out",
			},
			ninja.Rule{Name: "link", Command: "$cxx @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "$in -o $out $ldflags", Description: "LINK $out"},
			ninja.Rule{Name: "link_c", Command: "$cc @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "$in -o $out $ldflags", Description: "LINK $out"},
			ninja.Rule{Name: "link_shared", Command: "$cxx @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "-shared $in -o $out $ldflags", Description: "LINK_SHARED $out"},
			ninja.Rule{Name: "link_shared_c", Command: "$cc @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "-shared $in -o $out $ldflags", Description: "LINK_SHARED $out"},
			ninja.Rule{Name: "ar", Command: "$ar crs $out $in", Description: "AR $out"},
			ninja.Rule{Name: "header_unit", Command: "$cxx @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "$huflags", Description: "HEADER_UNIT $out"},
		)
	}
	*file = append(*file, ninja.Rule{
		Name: "fetch_dep", Command: "$clue deps fetch $dep", Description: "FETCH $dep",
	}, ninja.Rule{
		Name: "external_dep", Command: "$clue -quiet -variant $variant -target $platform deps build $dep", Description: "EXTERNAL $dep", Restat: true,
	}, ninja.Rule{
		Name: "custom", Command: "$clue -variant $variant -target $platform build $target", Description: "CUSTOM $target",
	})
}
