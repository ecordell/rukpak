#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ORG="github.com/ecordell"
MODULE="${ORG}/rukpak"

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

# create a temporary directory to generate code in and ensure we clean it up on exit
OUTPUT_BASE=$(mktemp -d)
trap 'rm -rf "${OUTPUT_BASE}"' ERR EXIT

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/ecordell/rukpak/pkg/apis/rukpak github.com/ecordell/rukpak/pkg/apis \
  "rukpak:v1alpha1" \
  --output-base "${OUTPUT_BASE}" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

# copy the generated resources
cp -Rv "${OUTPUT_BASE}/${MODULE}/." "${SCRIPT_ROOT}"