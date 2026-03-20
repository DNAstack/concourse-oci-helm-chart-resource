// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/Masterminds/semver/v3"
)

func mustSemver(t *testing.T, v string) *semver.Version {
	t.Helper()
	sv, err := semver.NewVersion(v)
	if err != nil {
		t.Fatalf("invalid semver %q: %v", v, err)
	}
	return sv
}

func TestCompareGitDescribeVersions(t *testing.T) {
	tests := []struct {
		name string
		v    string
		o    string
		want int
	}{
		{
			name: "equal versions",
			v:    "1.2.3",
			o:    "1.2.3",
			want: 0,
		},
		{
			name: "major ordering",
			v:    "2.0.0",
			o:    "1.0.0",
			want: 1,
		},
		{
			name: "major ordering is numeric not lexicographic",
			v:    "10.0.0",
			o:    "2.0.0",
			want: 1,
		},
		{
			name: "minor ordering",
			v:    "1.1.0",
			o:    "1.2.0",
			want: -1,
		},
		{
			name: "minor ordering is numeric not lexicographic",
			v:    "1.10.0",
			o:    "1.2.0",
			want: 1,
		},
		{
			name: "patch ordering",
			v:    "1.0.1",
			o:    "1.0.0",
			want: 1,
		},
		{
			name: "patch ordering is numeric not lexicographic",
			v:    "1.0.10",
			o:    "1.0.2",
			want: 1,
		},
		{
			name: "prerelease less than release",
			v:    "1.0.0-rc1",
			o:    "1.0.0",
			want: -1,
		},
		{
			name: "release greater than prerelease",
			v:    "1.0.0",
			o:    "1.0.0-rc1",
			want: 1,
		},
		{
			name: "git-describe numeric sort: 5 commits < 12 commits",
			v:    "1.0.0-5-gabcdef",
			o:    "1.0.0-12-g1234567",
			want: -1,
		},
		{
			name: "git-describe numeric sort: 12 commits > 5 commits",
			v:    "1.0.0-12-g1234567",
			o:    "1.0.0-5-gabcdef",
			want: 1,
		},
		{
			name: "equal prerelease",
			v:    "1.0.0-5-gabcdef",
			o:    "1.0.0-5-gabcdef",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := mustSemver(t, tt.v)
			o := mustSemver(t, tt.o)
			got := CompareGitDescribeVersions(v, o)
			if got != tt.want {
				t.Errorf("CompareGitDescribeVersions(%q, %q) = %d, want %d", tt.v, tt.o, got, tt.want)
			}
		})
	}
}

func TestSortBySemver(t *testing.T) {
	t.Run("sortBySemver should return empty slice when input is empty", func(t *testing.T) {
		result := sortBySemver([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %v", result)
		}
	})

	t.Run("sortBySemver should sort numerically not lexicographically", func(t *testing.T) {
		tags := []string{"1.10.0", "1.2.0", "1.0.0", "2.0.0", "1.1.0"}
		result := sortBySemver(tags)
		want := []string{"1.0.0", "1.1.0", "1.2.0", "1.10.0", "2.0.0"}
		if len(result) != len(want) {
			t.Fatalf("expected %d versions, got %d: %v", len(want), len(result), result)
		}
		for i, v := range result {
			if v.Original() != want[i] {
				t.Errorf("index %d: expected %q, got %q", i, want[i], v.Original())
			}
		}
	})

	t.Run("sortBySemver should preserve slice length when tags are not valid semver", func(t *testing.T) {
		// sortBySemver allocates len(allTags) and only fills valid entries,
		// so the tail of the slice contains zero-value semver (0.0.0).
		tags := []string{"not-a-version", "also-not"}
		result := sortBySemver(tags)
		// Slice length equals input length; zero-value entries sort first.
		if len(result) != len(tags) {
			t.Errorf("expected slice length %d, got %d", len(tags), len(result))
		}
	})

	t.Run("sortBySemver should sort git-describe prereleases numerically", func(t *testing.T) {
		tags := []string{"1.0.0-12-g1234567", "1.0.0-5-gabcdef"}
		result := sortBySemver(tags)
		if len(result) != 2 {
			t.Fatalf("expected 2 versions, got %d", len(result))
		}
		if result[0].Original() != "1.0.0-5-gabcdef" {
			t.Errorf("expected first to be 1.0.0-5-gabcdef, got %q", result[0].Original())
		}
		if result[1].Original() != "1.0.0-12-g1234567" {
			t.Errorf("expected second to be 1.0.0-12-g1234567, got %q", result[1].Original())
		}
	})
}
