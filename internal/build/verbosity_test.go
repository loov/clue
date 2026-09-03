package build

import (
	"testing"
)

func TestValidateVerbosityFlags_RejectsConflictingModes(t *testing.T) {
	tests := []struct {
		name    string
		quiet   bool
		verbose bool
		wantErr bool
	}{
		{
			name:    "both false is valid",
			quiet:   false,
			verbose: false,
			wantErr: false,
		},
		{
			name:    "only quiet is valid",
			quiet:   true,
			verbose: false,
			wantErr: false,
		},
		{
			name:    "only verbose is valid",
			quiet:   false,
			verbose: true,
			wantErr: false,
		},
		{
			name:    "both true is invalid",
			quiet:   true,
			verbose: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVerbosityFlags(tt.quiet, tt.verbose)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVerbosityFlags(%v, %v) error = %v, wantErr %v",
					tt.quiet, tt.verbose, err, tt.wantErr)
			}
		})
	}
}
