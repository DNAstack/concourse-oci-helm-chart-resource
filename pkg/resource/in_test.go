// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

func mustDigest(data []byte) digest.Digest {
	return digest.FromBytes(data)
}

func mustReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

// inTestRepository wraps memory.Store and adds a Tags method to satisfy
// the Repository interface. Resolve and Fetch delegate to the store directly.
type inTestRepository struct {
	*memory.Store
}

func (r *inTestRepository) Tags(_ context.Context, _ string, fn func(tags []string) error) error {
	return fn([]string{})
}

func (r *inTestRepository) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	return r.Store.Resolve(ctx, ref)
}

// buildChartRepo creates a memory store with a packed OCI manifest containing
// a helm chart layer, tagged with the given version.
func buildChartRepo(t *testing.T, tag string) (*inTestRepository, ocispec.Descriptor) {
	t.Helper()
	ctx := context.Background()
	store := memory.New()

	// Push a fake chart archive layer.
	chartContent := []byte("fake-chart-content")
	chartDesc := ocispec.Descriptor{
		MediaType: mediaTypeHelmChartContentArchive,
		Digest:    mustDigest(chartContent),
		Size:      int64(len(chartContent)),
	}
	if err := store.Push(ctx, chartDesc, mustReader(chartContent)); err != nil {
		t.Fatalf("failed to push chart layer: %v", err)
	}

	// Pack a manifest with the chart layer and an annotation.
	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_0, "", oras.PackManifestOptions{
		Layers: []ocispec.Descriptor{chartDesc},
		ManifestAnnotations: map[string]string{
			"org.example.chart": "test",
		},
	})
	if err != nil {
		t.Fatalf("failed to pack manifest: %v", err)
	}

	// Tag the manifest with the version.
	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		t.Fatalf("failed to tag manifest: %v", err)
	}

	return &inTestRepository{Store: store}, manifestDesc
}

func TestGetRequestValidate(t *testing.T) {
	t.Run("missing tag returns error", func(t *testing.T) {
		req := GetRequest{
			Source:  Source{Registry: "r.example.com", Repository: "repo", ChartName: "chart"},
			Version: Version{Tag: ""},
		}
		err := req.Validate()
		if err == nil {
			t.Error("expected error for missing tag, got nil")
		}
		if err != nil && err.Error() != "tag is required" {
			t.Errorf("expected 'tag is required', got %q", err.Error())
		}
	})

	t.Run("missing source registry returns error", func(t *testing.T) {
		req := GetRequest{
			Source:  Source{Repository: "repo", ChartName: "chart"},
			Version: Version{Tag: "1.0.0"},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for missing registry, got nil")
		}
	})

	t.Run("valid request passes", func(t *testing.T) {
		req := GetRequest{
			Source:  Source{Registry: "r.example.com", Repository: "repo", ChartName: "chart"},
			Version: Version{Tag: "1.0.0"},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})
}

func TestGet(t *testing.T) {
	tag := "1.0.0"
	source := Source{
		Registry:   "registry.example.com",
		Repository: "charts",
		ChartName:  "mychart",
	}

	t.Run("writes files to output dir and returns response", func(t *testing.T) {
		repo, manifestDesc := buildChartRepo(t, tag)
		outputDir := t.TempDir()

		req := GetRequest{Source: source, Version: Version{Tag: tag}}
		resp, err := Get(context.Background(), req, outputDir, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify response fields.
		if resp.Tag != tag {
			t.Errorf("expected tag %q, got %q", tag, resp.Tag)
		}
		if resp.Digest != manifestDesc.Digest.String() {
			t.Errorf("expected digest %q, got %q", manifestDesc.Digest.String(), resp.Digest)
		}

		// Verify the manifest JSON file was written.
		manifestFile := filepath.Join(outputDir, "mychart-1.0.0.json")
		if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
			t.Errorf("expected manifest file %s to exist", manifestFile)
		}

		// Verify the chart archive (.tgz) was written.
		chartFile := filepath.Join(outputDir, "mychart-1.0.0.tgz")
		if _, err := os.Stat(chartFile); os.IsNotExist(err) {
			t.Errorf("expected chart file %s to exist", chartFile)
		}

		// Verify chart file content.
		content, err := os.ReadFile(chartFile)
		if err != nil {
			t.Fatalf("failed to read chart file: %v", err)
		}
		if string(content) != "fake-chart-content" {
			t.Errorf("unexpected chart content: %q", string(content))
		}
	})

	t.Run("metadata from manifest annotations is returned", func(t *testing.T) {
		repo, _ := buildChartRepo(t, tag)
		outputDir := t.TempDir()

		req := GetRequest{Source: source, Version: Version{Tag: tag}}
		resp, err := Get(context.Background(), req, outputDir, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found := false
		for _, m := range resp.Metadata {
			if m.Name == "org.example.chart" && m.Value == "test" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected metadata 'org.example.chart=test', got: %v", resp.Metadata)
		}
	})
}
