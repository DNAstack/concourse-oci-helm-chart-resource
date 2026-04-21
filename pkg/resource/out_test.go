// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

// saveTestChart writes <name>-<version>.tgz into dir using the Helm SDK, so
// tests exercise Put against a real helm archive rather than synthetic bytes.
func saveTestChart(t *testing.T, dir, name, description, version string) {
	t.Helper()
	if _, err := chartutil.Save(&chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion:  "v2",
			Name:        name,
			Description: description,
			Version:     version,
		},
	}, dir); err != nil {
		t.Fatalf("save chart: %v", err)
	}
}

// fetchManifest resolves tag in target and decodes the manifest blob.
func fetchManifest(t *testing.T, target oras.ReadOnlyTarget, tag string) ocispec.Manifest {
	t.Helper()
	ctx := context.Background()
	desc, err := target.Resolve(ctx, tag)
	if err != nil {
		t.Fatalf("resolve tag %q: %v", tag, err)
	}
	rc, err := target.Fetch(ctx, desc)
	if err != nil {
		t.Fatalf("fetch manifest: %v", err)
	}
	defer rc.Close()
	var m ocispec.Manifest
	if err := json.NewDecoder(rc).Decode(&m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return m
}

func TestPutRequestValidate(t *testing.T) {
	t.Run("putRequest should fail validation when chart_dir is missing", func(t *testing.T) {
		req := PutRequest{
			Source: Source{Registry: "r.example.com", Repository: "repo", ChartName: "chart"},
			Params: PutParams{ChartDir: ""},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for missing chart_dir, got nil")
		}
	})

	t.Run("putRequest should fail validation when source fields are missing", func(t *testing.T) {
		req := PutRequest{
			Source: Source{},
			Params: PutParams{ChartDir: "charts"},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for missing source fields, got nil")
		}
	})

	t.Run("putRequest should pass validation when all fields are provided", func(t *testing.T) {
		req := PutRequest{
			Source: Source{Registry: "r.example.com", Repository: "repo", ChartName: "chart"},
			Params: PutParams{ChartDir: "charts"},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})
}

func TestPut(t *testing.T) {
	source := Source{
		Registry:   "registry.example.com",
		Repository: "charts",
		ChartName:  "mychart",
	}

	t.Run("put should push chart and return version with metadata", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveTestChart(t, chartDir, "mychart", "A test chart", "2.1.0")

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		resp, err := Put(context.Background(), req, inputDir, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Version.Tag != "2.1.0" {
			t.Errorf("expected tag %q, got %q", "2.1.0", resp.Version.Tag)
		}
		if resp.Version.Digest == "" {
			t.Error("expected non-empty digest")
		}

		foundChart, foundVersion := false, false
		for _, m := range resp.Metadata {
			if m.Name == "chart" && m.Value == "mychart" {
				foundChart = true
			}
			if m.Name == "version" && m.Value == "2.1.0" {
				foundVersion = true
			}
		}
		if !foundChart {
			t.Error("expected metadata with chart=mychart")
		}
		if !foundVersion {
			t.Error("expected metadata with version=2.1.0")
		}

		desc, err := target.Resolve(context.Background(), "2.1.0")
		if err != nil {
			t.Fatalf("failed to resolve tag in target store: %v", err)
		}
		if desc.Digest.String() != resp.Version.Digest {
			t.Errorf("target digest %q != response digest %q", desc.Digest.String(), resp.Version.Digest)
		}
	})

	t.Run("put should return error when no tgz files exist", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		if _, err := Put(context.Background(), req, inputDir, target); err == nil {
			t.Fatal("expected error for empty chart dir, got nil")
		}
	})

	t.Run("put should return error when multiple tgz files exist", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chartDir, "mychart-1.0.0.tgz"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chartDir, "mychart-2.0.0.tgz"), []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		if _, err := Put(context.Background(), req, inputDir, target); err == nil {
			t.Fatal("expected error for multiple tgz files, got nil")
		}
	})

	t.Run("put should return error when chart name in archive does not match source", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveTestChart(t, chartDir, "otherchart", "A test chart", "1.0.0")

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		if _, err := Put(context.Background(), req, inputDir, target); err == nil {
			t.Fatal("expected error for chart-name mismatch, got nil")
		}
	})

	t.Run("put should return error when tgz is not a valid helm chart", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chartDir, "mychart-1.0.0.tgz"), []byte("not a helm chart"), 0o644); err != nil {
			t.Fatal(err)
		}

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		if _, err := Put(context.Background(), req, inputDir, target); err == nil {
			t.Fatal("expected error for non-helm tgz, got nil")
		}
	})

	t.Run("put should use helm-compatible config mediatype by default", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveTestChart(t, chartDir, "mychart", "A test chart", "1.0.0")

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		resp, err := Put(context.Background(), req, inputDir, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		m := fetchManifest(t, target, resp.Version.Tag)
		if want := "application/vnd.cncf.helm.config.v1+json"; m.Config.MediaType != want {
			t.Errorf("config mediatype: got %q, want %q", m.Config.MediaType, want)
		}
	})

	t.Run("put should set OCI manifest annotations from Chart.yaml", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveTestChart(t, chartDir, "mychart", "Annotations test chart", "1.0.0")

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		resp, err := Put(context.Background(), req, inputDir, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		m := fetchManifest(t, target, resp.Version.Tag)
		want := map[string]string{
			ocispec.AnnotationTitle:       "mychart",
			ocispec.AnnotationDescription: "Annotations test chart",
			ocispec.AnnotationVersion:     "1.0.0",
		}
		for k, v := range want {
			if got := m.Annotations[k]; got != v {
				t.Errorf("annotation %q: got %q, want %q", k, got, v)
			}
		}
	})

	t.Run("put should embed chart metadata in config blob", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveTestChart(t, chartDir, "mychart", "A test chart", "1.0.0")

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		resp, err := Put(context.Background(), req, inputDir, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		m := fetchManifest(t, target, resp.Version.Tag)
		// A literal "{}" (size 2) means the push is lossy and consumers can't
		// recover name/version from the manifest alone.
		if m.Config.Size <= 2 {
			t.Errorf("config blob size is %d; expected chart metadata JSON, not empty {}", m.Config.Size)
		}

		rc, err := target.Fetch(context.Background(), m.Config)
		if err != nil {
			t.Fatalf("failed to fetch config blob: %v", err)
		}
		defer rc.Close()
		var cfg map[string]any
		if err := json.NewDecoder(rc).Decode(&cfg); err != nil {
			t.Fatalf("failed to decode config blob as JSON: %v", err)
		}
		if cfg["name"] != "mychart" {
			t.Errorf("config.name: got %v, want %q", cfg["name"], "mychart")
		}
		if cfg["version"] != "1.0.0" {
			t.Errorf("config.version: got %v, want %q", cfg["version"], "1.0.0")
		}
	})

	t.Run("put should use custom config mediatype when configured", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		saveTestChart(t, chartDir, "mychart", "A test chart", "1.0.0")

		customSource := Source{
			Registry:        "registry.example.com",
			Repository:      "charts",
			ChartName:       "mychart",
			ConfigMediaType: "application/vnd.cncf.helm.chart.v2+json",
		}

		target := memory.New()
		req := PutRequest{Source: customSource, Params: PutParams{ChartDir: "output"}}
		resp, err := Put(context.Background(), req, inputDir, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		m := fetchManifest(t, target, resp.Version.Tag)
		if want := "application/vnd.cncf.helm.chart.v2+json"; m.Config.MediaType != want {
			t.Errorf("config mediatype: got %q, want %q", m.Config.MediaType, want)
		}
	})
}
