#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

MODULE="github.com/ankrsinha/mini-task"

CODEGEN_PKG=$(go list -m -f '{{.Dir}}' k8s.io/code-generator)

source "${CODEGEN_PKG}/kube_codegen.sh"

# generate deepcopy
kube::codegen::gen_helpers \
  --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
  "${SCRIPT_ROOT}/pkg/apis"

# generate client, listers, informers
kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated" \
  --output-pkg "${MODULE}/pkg/generated" \
  --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
  "${SCRIPT_ROOT}/pkg/apis"
