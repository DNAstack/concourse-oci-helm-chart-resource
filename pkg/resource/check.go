// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

type (
	CheckRequest struct {
		Source  Source   `json:"source"`
		Version *Version `json:"version,omitempty"`
	}

	CheckResponse []Version
)

func (cr *CheckRequest) Validate() error {
	return cr.Source.Validate()
}

func Check(ctx context.Context, request CheckRequest) (*CheckResponse, error) {
	repo, err := newRepositoryForSource(ctx, request.Source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create repository client")
	}

	// Fetch repository tags
	allTags, err := registry.Tags(ctx, repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch tags")
	}

	// Sort tags by semver
	sortedSemvers := sortBySemver(allTags)

	// chop the list at the index of the requested version, if there was one
	if request.Version != nil {
		requestedVersion, err := semver.NewVersion(request.Version.Tag)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to parse semver in requested version %q", request.Version.Tag))
		}
		for i, version := range sortedSemvers {
			if version.GreaterThanEqual(requestedVersion) {
				sortedSemvers = sortedSemvers[i:]
				break
			}
		}
	} else {
		// if no version was requested, return the latest 10 versions
		startIndex := len(sortedSemvers) - 10
		if startIndex < 0 {
			startIndex = 0
		}
		sortedSemvers = sortedSemvers[startIndex:]
	}

	if len(sortedSemvers) == 0 {
		return nil, fmt.Errorf("no latest tag found for source %s", request.Source.String())
	}

	resolvedVersions, err := resolveImageDigests(ctx, sortedSemvers, repo)
	if err != nil {
		return nil, err
	}
	return &resolvedVersions, nil
}

func sortBySemver(allTags []string) []semver.Version {
	allVersions := make([]semver.Version, len(allTags))
	var j = 0
	for _, tag := range allTags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping tag %q because it does not look like a semver\n", tag)
			continue
		}
		allVersions[j] = *v
		j++
	}
	slices.SortStableFunc(allVersions, func(i, j semver.Version) int {
		return i.Compare(&j)
	})
	return allVersions
}

func resolveImageDigests(ctx context.Context, sortedSemvers []semver.Version, repo *remote.Repository) (CheckResponse, error) {
	resolvedVersions := make(CheckResponse, len(sortedSemvers))
	for i, version := range sortedSemvers {
		digest, err := getDigestForTag(ctx, repo, version.Original())
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to fetch digest for latest tag %q (parsed as %s)", version.Original(), version.String()))
		}
		resolvedVersions[i] = Version{Tag: version.Original(), Digest: digest}
	}
	return resolvedVersions, nil
}
