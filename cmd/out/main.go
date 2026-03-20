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
	var req resource.PutRequest

	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal request: %s\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing arguments")
		os.Exit(1)
	}
	if err := req.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid source configuration: %s\n", err)
		os.Exit(1)
	}
	inputDir := os.Args[1]
	ctx := context.Background()
	repo, err := resource.NewRepositoryForSource(ctx, req.Source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create repository: %s\n", err)
		os.Exit(1)
	}
	response, err := resource.Put(ctx, req, inputDir, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "put failed: %s\n", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %s\n", err)
		os.Exit(1)
	}
}
