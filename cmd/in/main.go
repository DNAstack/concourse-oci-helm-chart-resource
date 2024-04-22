// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/cloudoperators/concourse-oci-helm-chart-resource/pkg/resource"
)

func main() {
	var req resource.GetRequest

	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&req); err != nil {
		log.Fatalf("failed to unmarshal request: %s", err)
	}

	if len(os.Args) < 2 {
		log.Fatalf("missing arguments")
	}
	outputDir := os.Args[1]
	if err := req.Validate(); err != nil {
		log.Fatalf("invalid source configuration: %s", err)
	}
	response, err := resource.Get(context.Background(), req, outputDir)
	if err != nil {
		log.Fatalf("get failed: %s", err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		log.Fatalf("failed to marshal response: %s", err)
	}
}
