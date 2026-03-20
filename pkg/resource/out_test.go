// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/memory"
)

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
		chartContent := []byte("fake-chart-archive")
		if err := os.WriteFile(filepath.Join(chartDir, "mychart-2.1.0.tgz"), chartContent, 0o644); err != nil {
			t.Fatal(err)
		}

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

		// Verify metadata
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

		// Verify chart was pushed to target by resolving the tag
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
		_, err := Put(context.Background(), req, inputDir, target)
		if err == nil {
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
		_, err := Put(context.Background(), req, inputDir, target)
		if err == nil {
			t.Fatal("expected error for multiple tgz files, got nil")
		}
	})

	t.Run("put should return error when tgz filename does not match chart name", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chartDir, "otherchart-1.0.0.tgz"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		_, err := Put(context.Background(), req, inputDir, target)
		if err == nil {
			t.Fatal("expected error for wrong filename prefix, got nil")
		}
	})

	t.Run("put should use helm-compatible config mediatype by default", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chartDir, "mychart-1.0.0.tgz"), []byte("chart"), 0o644); err != nil {
			t.Fatal(err)
		}

		target := memory.New()
		req := PutRequest{Source: source, Params: PutParams{ChartDir: "output"}}
		resp, err := Put(context.Background(), req, inputDir, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		manifestDesc, err := target.Resolve(context.Background(), resp.Version.Tag)
		if err != nil {
			t.Fatalf("failed to resolve tag: %v", err)
		}
		rc, err := target.Fetch(context.Background(), manifestDesc)
		if err != nil {
			t.Fatalf("failed to fetch manifest: %v", err)
		}
		defer rc.Close()
		var manifest ocispec.Manifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			t.Fatalf("failed to decode manifest: %v", err)
		}

		expected := "application/vnd.cncf.helm.config.v1+json"
		if manifest.Config.MediaType != expected {
			t.Errorf("expected config mediatype %q, got %q", expected, manifest.Config.MediaType)
		}
	})

	t.Run("put should use custom config mediatype when configured", func(t *testing.T) {
		inputDir := t.TempDir()
		chartDir := filepath.Join(inputDir, "output")
		if err := os.MkdirAll(chartDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chartDir, "mychart-1.0.0.tgz"), []byte("chart"), 0o644); err != nil {
			t.Fatal(err)
		}

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

		manifestDesc, err := target.Resolve(context.Background(), resp.Version.Tag)
		if err != nil {
			t.Fatalf("failed to resolve tag: %v", err)
		}
		rc, err := target.Fetch(context.Background(), manifestDesc)
		if err != nil {
			t.Fatalf("failed to fetch manifest: %v", err)
		}
		defer rc.Close()
		var manifest ocispec.Manifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			t.Fatalf("failed to decode manifest: %v", err)
		}

		expected := "application/vnd.cncf.helm.chart.v2+json"
		if manifest.Config.MediaType != expected {
			t.Errorf("expected config mediatype %q, got %q", expected, manifest.Config.MediaType)
		}
	})
}
