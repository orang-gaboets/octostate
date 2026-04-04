package safety

import (
	"errors"
	"testing"
)

func TestRequireYesOrDryRun(t *testing.T) {
	tests := []struct {
		name    string
		yes     bool
		dryRun  bool
		wantErr bool
	}{
		{name: "yes", yes: true, dryRun: false, wantErr: false},
		{name: "dry-run", yes: false, dryRun: true, wantErr: false},
		{name: "both", yes: true, dryRun: true, wantErr: false},
		{name: "neither", yes: false, dryRun: false, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireYesOrDryRun(tc.yes, tc.dryRun)
			if tc.wantErr {
				if !errors.Is(err, ErrConfirmationRequired) {
					t.Fatalf("expected ErrConfirmationRequired, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
