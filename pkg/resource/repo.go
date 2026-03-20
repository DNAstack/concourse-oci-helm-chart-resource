// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Repository is the interface that Check and Get need from the OCI registry.
type Repository interface {
	oras.ReadOnlyTarget
	registry.TagLister
	Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error)
}

var allowedMediaTypes = []string{
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json, ",
	"application/vnd.oci.image.index.v1+json",
	"*/*",
}

func NewRepositoryForSource(_ context.Context, s Source) (*remote.Repository, error) {
	repo, err := remote.NewRepository(s.String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create repository from source %s", s.String())
	}
	repo.ManifestMediaTypes = allowedMediaTypes

	client := &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
	}
	// 	Set up credentials from docker.
	storeOpts := credentials.StoreOptions{}
	credStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		panic(err)
	}
	client.Credential = credentials.Credential(credStore)

	// Set up basic auth credentials.
	if s.AuthUsername != "" && s.AuthPassword != "" {
		client.Credential = auth.StaticCredential(s.Registry, auth.Credential{
			Username: s.AuthUsername,
			Password: s.AuthPassword,
		})
	}
	repo.Client = client
	return repo, nil
}

func getDigestForTag(ctx context.Context, repo Repository, source, tag string) (string, error) {
	desc, err := repo.Resolve(ctx, fmt.Sprintf("%s:%s", source, tag))
	if err != nil {
		return "", err
	}
	return desc.Digest.String(), nil
}
