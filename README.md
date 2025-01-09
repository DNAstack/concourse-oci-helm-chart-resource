Concourse resource for artifacts in an OCI registry
===================================================
[![REUSE status](https://api.reuse.software/badge/github.com/cloudoperators/concourse-oci-helm-chart-resource)](https://api.reuse.software/info/github.com/cloudoperators/concourse-oci-helm-chart-resource)

Fetches, verifies and publishes Helm Charts from a running OCI registry.

## Installation

Add a new resource type to your Concourse CI pipeline:

```yaml
resource_types:
- name: oci-registry
  type: docker-image
  source:
    repository: ghcr.io/cloudoperators/concourse-oci-helm-chart-resource
    tag: f932b76 # Replace with the latest stable release.
```

## Configuration

```
resources:
  - name: my.chart
    type: oci-registry
    source:
      registry: ghcr.io
      repository: cloudoperators/all-my-helm-charts
      chart_name: my-chart
```

#### Authentication

The resource supports two ways of authenticating against the registry:
* By default, the docker credential store is used.
* Use the `auth_username` and `auth_password` parameters within the `source` block for basic authentication.

## Behavior

The resource implements the `check` and `in` action.

### check: Check for new versions of the Helm chart

Checks for new versions of the specified Helm chart. Returns the latest version of the Helm chart based on semantic versioning.

### in: Download the Helm chart and the metadata file

Places the packaged Helm chart and the metadata file in the destination directory following the `<$>chart_name>-<chart_version>.{tgz|json}` naming convention.

### out

* Currently not supported. Use `helm push` to push Helm charts to an OCI registry.
