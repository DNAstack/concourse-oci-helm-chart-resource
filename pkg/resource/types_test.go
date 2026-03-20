// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"
)

func TestSourceValidate(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		wantErr string
	}{
		{
			name:    "missing registry",
			source:  Source{Repository: "myrepo", ChartName: "mychart"},
			wantErr: "registry cannot be empty",
		},
		{
			name:    "missing repository",
			source:  Source{Registry: "registry.example.com", ChartName: "mychart"},
			wantErr: "repository cannot be empty",
		},
		{
			name:    "missing chart_name",
			source:  Source{Registry: "registry.example.com", Repository: "myrepo"},
			wantErr: "chart_name cannot be empty",
		},
		{
			name:   "all fields present",
			source: Source{Registry: "registry.example.com", Repository: "myrepo", ChartName: "mychart"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %q, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestSourceString(t *testing.T) {
	s := Source{
		Registry:   "registry.example.com",
		Repository: "myrepo",
		ChartName:  "mychart",
	}
	want := "registry.example.com/myrepo/mychart"
	got := s.String()
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
