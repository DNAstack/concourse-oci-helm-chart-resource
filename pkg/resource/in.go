// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

const (
	mediaTypeHelmChartContentArchive = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
	mediaTypeHelmChartJSON           = "application/vnd.cncf.helm.chart.v2+json"
)

type (
	GetRequest struct {
		Source  Source  `json:"source"`
		Version Version `json:"version"`
	}

	GetResponse struct {
		Tag      string         `json:"tag"`
		Digest   string         `json:"digest"`
		Metadata []MetadataItem `json:"metadata,omitempty"`
	}
)

func (gr *GetRequest) Validate() error {
	if gr.Version.Tag == "" {
		return errors.New("tag is required")
	}
	return gr.Source.Validate()
}

func Get(ctx context.Context, request GetRequest, outputDir string) (*GetResponse, error) {
	repo, err := newRepositoryForSource(ctx, request.Source)
	if err != nil {
		return nil, err
	}

	store := memory.New()
	desc, err := oras.Copy(ctx, repo, request.Version.Tag, store, request.Version.Tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download chart %s:%s", request.Source.String(), request.Version.Tag)
	}

	filenameWithoutExtension := fmt.Sprintf("%s-%s", request.Source.ChartName, request.Version.Tag)
	manifestData, err := fetchFromStoreAndPersistFile(ctx, store, desc, path.Join(outputDir, filenameWithoutExtension+".json"))
	if err != nil {
		return nil, err
	}
	var manifestDescriptor ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifestDescriptor); err != nil {
		return nil, err
	}

	// Extract metadata from all manifest annotations
	var metadata []MetadataItem
	if manifestDescriptor.Annotations != nil {
		for key, value := range manifestDescriptor.Annotations {
			metadata = append(metadata, MetadataItem{
				Name:  key,
				Value: value,
			})
		}
	}

	// Find different layers.
	for _, layer := range manifestDescriptor.Layers {
		var fileExtension string
		switch layer.MediaType {
		case mediaTypeHelmChartContentArchive:
			fileExtension = ".tgz"
		case mediaTypeHelmChartJSON:
			fileExtension = ".json"
		default:
			continue
		}

		// Persists the respective layer.
		if _, err := fetchFromStoreAndPersistFile(ctx, store, layer, path.Join(outputDir, filenameWithoutExtension+fileExtension)); err != nil {
			return nil, err
		}
	}

	return &GetResponse{
		Tag:      request.Version.Tag,
		Digest:   desc.Digest.String(),
		Metadata: metadata,
	}, nil
}

func fetchFromStoreAndPersistFile(ctx context.Context, store oras.ReadOnlyTarget, descriptor ocispec.Descriptor, filename string) ([]byte, error) {
	r, err := store.Fetch(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	layerBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filename, layerBytes, os.ModePerm)
	return layerBytes, err
}
