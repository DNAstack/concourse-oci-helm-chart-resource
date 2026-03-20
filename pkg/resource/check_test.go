// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

// testRepository implements Repository using an in-memory store for content,
// with configurable tags and digest mappings.
type testRepository struct {
	*memory.Store
	tags    []string
	digests map[string]ocispec.Descriptor
}

func (r *testRepository) Tags(ctx context.Context, last string, fn func(tags []string) error) error {
	return fn(r.tags)
}

func (r *testRepository) Resolve(_ context.Context, ref string) (ocispec.Descriptor, error) {
	desc, ok := r.digests[ref]
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("not found: %s", ref)
	}
	return desc, nil
}

func newTestRepo(tags []string, source string) *testRepository {
	digests := make(map[string]ocispec.Descriptor, len(tags))
	for _, tag := range tags {
		fakeDigest := digest.FromString(tag)
		digests[fmt.Sprintf("%s:%s", source, tag)] = ocispec.Descriptor{
			Digest: fakeDigest,
		}
	}
	return &testRepository{
		Store:   memory.New(),
		tags:    tags,
		digests: digests,
	}
}

func TestCheckRequestValidate(t *testing.T) {
	t.Run("checkRequest should fail validation when source is invalid", func(t *testing.T) {
		req := CheckRequest{
			Source: Source{Repository: "myrepo", ChartName: "mychart"}, // missing registry
		}
		err := req.Validate()
		if err == nil {
			t.Error("expected validation error, got nil")
		}
	})

	t.Run("checkRequest should pass validation when source is valid", func(t *testing.T) {
		req := CheckRequest{
			Source: Source{Registry: "r.example.com", Repository: "myrepo", ChartName: "mychart"},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})
}

func TestCheck(t *testing.T) {
	source := Source{
		Registry:   "registry.example.com",
		Repository: "charts",
		ChartName:  "mychart",
	}
	sourceStr := source.String() // "registry.example.com/charts/mychart"

	t.Run("check should return latest 10 revisions when no starting revision is specified", func(t *testing.T) {
		tags := []string{"1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0", "1.5.0", "1.6.0", "1.7.0", "1.8.0", "1.9.0", "2.0.0"}
		repo := newTestRepo(tags, sourceStr)
		req := CheckRequest{Source: source}

		resp, err := Check(context.Background(), req, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should return latest 10 (1.1.0 through 2.0.0)
		if len(*resp) != 10 {
			t.Errorf("expected 10 versions, got %d", len(*resp))
		}
		// First should be 1.1.0, last should be 2.0.0
		if (*resp)[0].Tag != "1.1.0" {
			t.Errorf("expected first tag 1.1.0, got %q", (*resp)[0].Tag)
		}
		if (*resp)[len(*resp)-1].Tag != "2.0.0" {
			t.Errorf("expected last tag 2.0.0, got %q", (*resp)[len(*resp)-1].Tag)
		}
	})

	t.Run("check should return all revisions when fewer than 10 exist", func(t *testing.T) {
		tags := []string{"1.0.0", "1.1.0", "1.2.0"}
		repo := newTestRepo(tags, sourceStr)
		req := CheckRequest{Source: source}

		resp, err := Check(context.Background(), req, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(*resp) != 3 {
			t.Errorf("expected 3 versions, got %d", len(*resp))
		}
	})

	t.Run("check should return versions from cursor onwards when starting revision is specified", func(t *testing.T) {
		tags := []string{"1.0.0", "1.1.0", "1.2.0", "1.3.0", "2.0.0"}
		repo := newTestRepo(tags, sourceStr)
		req := CheckRequest{
			Source:  source,
			Version: &Version{Tag: "1.2.0"},
		}

		resp, err := Check(context.Background(), req, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return 1.2.0, 1.3.0, 2.0.0
		if len(*resp) != 3 {
			t.Errorf("expected 3 versions, got %d", len(*resp))
		}
		if (*resp)[0].Tag != "1.2.0" {
			t.Errorf("expected first tag 1.2.0, got %q", (*resp)[0].Tag)
		}
	})

	t.Run("check should return versions in semver order when tags are unordered", func(t *testing.T) {
		// Provide tags out of order
		tags := []string{"2.0.0", "1.0.0", "1.1.0"}
		repo := newTestRepo(tags, sourceStr)
		req := CheckRequest{Source: source}

		resp, err := Check(context.Background(), req, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"1.0.0", "1.1.0", "2.0.0"}
		for i, v := range *resp {
			if v.Tag != want[i] {
				t.Errorf("index %d: expected %q, got %q", i, want[i], v.Tag)
			}
		}
	})

	t.Run("check should populate digest in response", func(t *testing.T) {
		tags := []string{"1.0.0"}
		repo := newTestRepo(tags, sourceStr)
		req := CheckRequest{Source: source}

		resp, err := Check(context.Background(), req, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if (*resp)[0].Digest == "" {
			t.Error("expected digest to be populated")
		}
	})

	t.Run("check should return error when no tags exist", func(t *testing.T) {
		repo := newTestRepo([]string{}, sourceStr)
		req := CheckRequest{Source: source}

		_, err := Check(context.Background(), req, repo)
		if err == nil {
			t.Error("expected error for empty tag list, got nil")
		}
	})
}
