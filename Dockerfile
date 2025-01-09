FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.22 AS build

WORKDIR /concourse-oci-helm-chart-resource
COPY . .
RUN make build

FROM --platform=${BUILDPLATFORM:-linux/amd64} alpine:3.20.3 AS run

# upgrade all installed packages to fix potential CVEs in advance
RUN apk upgrade --no-cache --no-progress \
  && apk add --no-cache --no-progress --upgrade ca-certificates \
  && apk del --no-cache --no-progress apk-tools alpine-keys

# Required by concourse resource. Copy explicitly.
COPY --from=build /concourse-oci-helm-chart-resource/bin/check /opt/resource/check
RUN chmod +x /opt/resource/check

COPY --from=build /concourse-oci-helm-chart-resource/bin/in /opt/resource/in
RUN chmod +x /opt/resource/in

COPY --from=build /concourse-oci-helm-chart-resource/bin/out /opt/resource/out
RUN chmod +x /opt/resource/out
