#!/bin/bash

set -euox pipefail

podman pull docker.io/library/alpine
podman pull docker.io/library/alpine:edge
podman pull docker.io/library/alpine:3.15

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

pushd ${SCRIPT_DIR}

IMG="localhost/container-layer-sizes-testimage"

buildah bud --target res1 --layers -t "${IMG}:3.0" .
buildah bud --target res2 --layers -t "${IMG}:latest" .

mkdir -p "${1}"/{latest,3.0}/
for tag in "latest" "3.0"; do
    buildah push "${IMG}:${tag}" docker-archive:"${1}/${tag}/testimage.tar.gz"
    buildah push "${IMG}:${tag}" oci:"${1}/${tag}/testimage:${tag}"
done

popd
