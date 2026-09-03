package build

import (
	"runtime"
	"strings"
	"testing"
)

func TestHostPlatform_MatchesRuntime(t *testing.T) {
	host := HostPlatform()

	if host.OS == "" {
		t.Error("HostPlatform() returned empty OS")
	}

	if host.Arch == "" {
		t.Error("HostPlatform() returned empty Arch")
	}

	// Should match runtime constants
	if host.OS != runtime.GOOS {
		t.Errorf("HostPlatform() OS = %s, want %s", host.OS, runtime.GOOS)
	}

	if host.Arch != runtime.GOARCH {
		t.Errorf("HostPlatform() Arch = %s, want %s", host.Arch, runtime.GOARCH)
	}
}

func TestPlatformString_JoinsOSAndArchitecture(t *testing.T) {
	tests := []struct {
		name string
		p    Platform
		want string
	}{
		{
			name: "linux-amd64",
			p:    Platform{OS: "linux", Arch: "amd64"},
			want: "linux-amd64",
		},
		{
			name: "linux-arm64",
			p:    Platform{OS: "linux", Arch: "arm64"},
			want: "linux-arm64",
		},
		{
			name: "darwin-amd64",
			p:    Platform{OS: "darwin", Arch: "amd64"},
			want: "darwin-amd64",
		},
		{
			name: "darwin-arm64",
			p:    Platform{OS: "darwin", Arch: "arm64"},
			want: "darwin-arm64",
		},
		{
			name: "windows-amd64",
			p:    Platform{OS: "windows", Arch: "amd64"},
			want: "windows-amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.String()
			if got != tt.want {
				t.Errorf("Platform.String() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseTarget_AcceptsSupportedPlatforms(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Platform
	}{
		{
			name:  "linux-amd64",
			input: "linux-amd64",
			want:  Platform{OS: "linux", Arch: "amd64"},
		},
		{
			name:  "linux-arm64",
			input: "linux-arm64",
			want:  Platform{OS: "linux", Arch: "arm64"},
		},
		{
			name:  "darwin-amd64",
			input: "darwin-amd64",
			want:  Platform{OS: "darwin", Arch: "amd64"},
		},
		{
			name:  "darwin-arm64",
			input: "darwin-arm64",
			want:  Platform{OS: "darwin", Arch: "arm64"},
		},
		{
			name:  "windows-amd64",
			input: "windows-amd64",
			want:  Platform{OS: "windows", Arch: "amd64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if err != nil {
				t.Fatalf("ParseTarget(%s) returned error: %v", tt.input, err)
			}

			if got.OS != tt.want.OS {
				t.Errorf("ParseTarget(%s) OS = %s, want %s", tt.input, got.OS, tt.want.OS)
			}

			if got.Arch != tt.want.Arch {
				t.Errorf("ParseTarget(%s) Arch = %s, want %s", tt.input, got.Arch, tt.want.Arch)
			}
		})
	}
}

func TestParseTarget_RejectsMalformedAndUnsupportedPlatforms(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantErrMsg string
	}{
		{
			name:       "invalid - no separator",
			input:      "invalid",
			wantErrMsg: "invalid target format",
		},
		{
			name:       "missing arch",
			input:      "linux",
			wantErrMsg: "invalid target format",
		},
		{
			name:       "too many parts",
			input:      "linux-arm64-extra",
			wantErrMsg: "invalid target format",
		},
		{
			name:       "empty string",
			input:      "",
			wantErrMsg: "invalid target format",
		},
		{
			name:       "unsupported - freebsd",
			input:      "freebsd-amd64",
			wantErrMsg: "unsupported target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTarget(tt.input)
			if err == nil {
				t.Fatalf("ParseTarget(%s) expected error containing %q, got nil", tt.input, tt.wantErrMsg)
			}

			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("ParseTarget(%s) error = %q, want error containing %q", tt.input, err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestIsSupportedTarget_RecognizesSupportedMatrix(t *testing.T) {
	tests := []struct {
		name      string
		platform  Platform
		supported bool
	}{
		// Supported platforms
		{
			name:      "linux-amd64 supported",
			platform:  Platform{OS: "linux", Arch: "amd64"},
			supported: true,
		},
		{
			name:      "linux-arm64 supported",
			platform:  Platform{OS: "linux", Arch: "arm64"},
			supported: true,
		},
		{
			name:      "darwin-amd64 supported",
			platform:  Platform{OS: "darwin", Arch: "amd64"},
			supported: true,
		},
		{
			name:      "darwin-arm64 supported",
			platform:  Platform{OS: "darwin", Arch: "arm64"},
			supported: true,
		},
		// Unsupported platforms
		{
			name:      "windows-amd64 supported",
			platform:  Platform{OS: "windows", Arch: "amd64"},
			supported: true,
		},
		{
			name:      "freebsd-amd64 unsupported",
			platform:  Platform{OS: "freebsd", Arch: "amd64"},
			supported: false,
		},
		{
			name:      "linux-386 unsupported",
			platform:  Platform{OS: "linux", Arch: "386"},
			supported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSupportedTarget(tt.platform)
			if got != tt.supported {
				t.Errorf("IsSupportedTarget(%s) = %v, want %v", tt.platform.String(), got, tt.supported)
			}
		})
	}
}

func TestPlatformIsCrossCompile_ComparesWithHost(t *testing.T) {
	host := HostPlatform()

	tests := []struct {
		name  string
		p     Platform
		cross bool
	}{
		{
			name:  "host platform is not cross-compile",
			p:     host,
			cross: false,
		},
		{
			name:  "different OS is cross-compile",
			p:     Platform{OS: "different", Arch: host.Arch},
			cross: true,
		},
		{
			name:  "different arch is cross-compile",
			p:     Platform{OS: host.OS, Arch: "different"},
			cross: true,
		},
		{
			name:  "both different is cross-compile",
			p:     Platform{OS: "different", Arch: "different"},
			cross: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.IsCrossCompile()
			if got != tt.cross {
				t.Errorf("Platform.IsCrossCompile() = %v, want %v (host=%s, target=%s)",
					got, tt.cross, host.String(), tt.p.String())
			}
		})
	}
}
