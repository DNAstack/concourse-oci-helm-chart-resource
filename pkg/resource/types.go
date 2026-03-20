// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"errors"
	"fmt"
)

type Source struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	ChartName  string `json:"chart_name"`

	AuthUsername string `json:"auth_username,omitempty"`
	AuthPassword string `json:"auth_password,omitempty"`

	ConfigMediaType string `json:"config_media_type,omitempty"`
}

func (s *Source) GetConfigMediaType() string {
	if s.ConfigMediaType != "" {
		return s.ConfigMediaType
	}
	return "application/vnd.cncf.helm.config.v1+json"
}

func (s *Source) Validate() error {
	if s.Registry == "" {
		return errors.New("registry cannot be empty")
	}
	if s.Repository == "" {
		return errors.New("repository cannot be empty")
	}
	if s.ChartName == "" {
		return errors.New("chart_name cannot be empty")
	}
	return nil
}

func (s *Source) String() string {
	return fmt.Sprintf("%s/%s/%s", s.Registry, s.Repository, s.ChartName)
}

type Version struct {
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

type MetadataItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
