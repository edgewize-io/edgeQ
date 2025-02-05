#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -x

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
CODEGEN_VERSION=$(cd "${SCRIPT_ROOT}" && grep 'k8s.io/code-generator' go.mod | grep -v '=>' | awk '{print $2}')

# For debugging purposes
echo "Codegen version ${CODEGEN_VERSION}"

if [ -z "${GOPATH:-}" ]; then
    GOPATH=$(go env GOPATH)
    export GOPATH
fi
CODEGEN_PKG="$GOPATH/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}"
THIS_PKG="github.com/edgewize/edgeQ"

chmod +x "${CODEGEN_PKG}/generate-groups.sh"
chmod +x "${CODEGEN_PKG}/generate-internal-groups.sh"

"${CODEGEN_PKG}/generate-groups.sh" \
    "deepcopy,client,informer,lister" \
    "${THIS_PKG}/pkg/client" \
    "${THIS_PKG}/pkg/apis" \
    "apps:v1alpha1" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    --output-base "$(pwd)/" \

rm -rf ./pkg/client
mv ${THIS_PKG}/pkg/client ./pkg
mv ${THIS_PKG}/pkg/apis/apps/v1alpha1/* ./pkg/apis/apps/v1alpha1/
rm -rf "./github.com"