package version

import (
	"testing"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

func TestIsVersionHigher(t *testing.T) {
	tests := []struct {
		name        string
		newVersion  documentationv1alpha1.DocumentVersion
		prevVersion documentationv1alpha1.DocumentVersion
		wantHigher  bool
		wantErr     bool
	}{
		{
			name:        "patch higher",
			newVersion:  "v1.0.1",
			prevVersion: "v1.0.0",
			wantHigher:  true,
		},
		{
			name:        "equal versions",
			newVersion:  "v1.2.3",
			prevVersion: "v1.2.3",
			wantHigher:  false,
		},
		{
			name:        "minor lower",
			newVersion:  "v1.1.0",
			prevVersion: "v1.2.0",
			wantHigher:  false,
		},
		{
			name:        "major higher",
			newVersion:  "v2.0.0",
			prevVersion: "v1.9.9",
			wantHigher:  true,
		},
		{
			name:        "invalid new",
			newVersion:  "1.0.0", // missing leading v
			prevVersion: "v0.9.0",
			wantErr:     true,
		},
		{
			name:        "invalid prev",
			newVersion:  "v1.0.0",
			prevVersion: "v1.0", // not three segments
			wantErr:     true,
		},
		{
			name:        "minor higher multiple digits",
			newVersion:  "v1.10.0",
			prevVersion: "v1.2.99",
			wantHigher:  true,
		},
		{
			name:        "patch lower with multiple digits",
			newVersion:  "v1.2.10",
			prevVersion: "v1.2.11",
			wantHigher:  false,
		},
		{
			name:        "major higher multiple digits",
			newVersion:  "v11.0.0",
			prevVersion: "v2.0.0",
			wantHigher:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHigher, err := IsVersionHigher(tt.newVersion, tt.prevVersion)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got %v", tt.wantErr, err)
			}
			if err == nil && gotHigher != tt.wantHigher {
				t.Fatalf("expected higher=%v, got %v", tt.wantHigher, gotHigher)
			}
		})
	}
}
