// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart/loader"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

type (
	PutRequest struct {
		Source Source    `json:"source"`
		Params PutParams `json:"params"`
	}

	PutParams struct {
		ChartDir string `json:"chart_dir"`
	}

	PutResponse struct {
		Version  Version        `json:"version"`
		Metadata []MetadataItem `json:"metadata,omitempty"`
	}
)

func (pr *PutRequest) Validate() error {
	if pr.Params.ChartDir == "" {
		return errors.New("params.chart_dir is required")
	}
	return pr.Source.Validate()
}

func Put(ctx context.Context, request PutRequest, inputDir string, target oras.Target) (*PutResponse, error) {
	chartDir := filepath.Join(inputDir, request.Params.ChartDir)
	matches, err := filepath.Glob(filepath.Join(chartDir, "*.tgz"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to glob for chart packages")
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no .tgz files found in %s", chartDir)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple .tgz files found in %s, expected exactly one", chartDir)
	}

	chartContent, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to read chart file")
	}

	loadedChart, err := loader.LoadArchive(bytes.NewReader(chartContent))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load chart archive")
	}

	if loadedChart.Metadata.Name != request.Source.ChartName {
		return nil, fmt.Errorf("chart name %q in archive does not match source chart_name %q",
			loadedChart.Metadata.Name, request.Source.ChartName)
	}
	tag := loadedChart.Metadata.Version

	configContent, err := json.Marshal(loadedChart.Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal chart metadata")
	}

	fmt.Fprintf(os.Stderr, "pushing %s version %s to %s\n", request.Source.ChartName, tag, request.Source.String())

	store := memory.New()

	// Push chart layer
	chartDesc := ocispec.Descriptor{
		MediaType: mediaTypeHelmChartContentArchive,
		Digest:    digest.FromBytes(chartContent),
		Size:      int64(len(chartContent)),
	}
	if err := store.Push(ctx, chartDesc, bytes.NewReader(chartContent)); err != nil {
		return nil, errors.Wrap(err, "failed to push chart layer to store")
	}

	// Push helm chart config blob (Chart.yaml serialized as JSON)
	configDesc := ocispec.Descriptor{
		MediaType: request.Source.GetConfigMediaType(),
		Digest:    digest.FromBytes(configContent),
		Size:      int64(len(configContent)),
	}
	if err := store.Push(ctx, configDesc, bytes.NewReader(configContent)); err != nil {
		return nil, errors.Wrap(err, "failed to push config to store")
	}

	// Pack OCI manifest with annotations that `helm push` would set, so
	// consumers can resolve chart name/description/version without fetching
	// and unpacking the chart archive.
	packOpts := oras.PackManifestOptions{
		Layers:           []ocispec.Descriptor{chartDesc},
		ConfigDescriptor: &configDesc,
		ManifestAnnotations: map[string]string{
			ocispec.AnnotationTitle:       loadedChart.Metadata.Name,
			ocispec.AnnotationDescription: loadedChart.Metadata.Description,
			ocispec.AnnotationVersion:     loadedChart.Metadata.Version,
		},
	}
	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", packOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack manifest")
	}

	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		return nil, errors.Wrap(err, "failed to tag manifest")
	}

	// Push to remote registry
	desc, err := oras.Copy(ctx, store, tag, target, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to push chart %s:%s", request.Source.String(), tag)
	}

	fmt.Fprintf(os.Stderr, "pushed %s:%s (digest: %s)\n", request.Source.String(), tag, desc.Digest.String())

	return &PutResponse{
		Version: Version{
			Tag:    tag,
			Digest: desc.Digest.String(),
		},
		Metadata: []MetadataItem{
			{Name: "chart", Value: request.Source.ChartName},
			{Name: "version", Value: tag},
		},
	}, nil
}
