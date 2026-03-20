// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudoperators/concourse-oci-helm-chart-resource/pkg/resource"
)

func main() {
	var req resource.CheckRequest
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal request: %s\n", err)
	}

	if err := req.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid source configuration: %s\n", err)
	}

	ctx := context.Background()
	repo, err := resource.NewRepositoryForSource(ctx, req.Source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create repository client: %s\n", err)
		os.Exit(1)
	}

	resp, err := resource.Check(ctx, req, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid source configuration: %s\n", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %s\n", err)
	}
}
