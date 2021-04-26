set -o nounset
set -o errexit
set -o pipefail

export INSTALL_HOME=/opt/dragonfly
export INSTALL_CDN_PATH=df-cdn
export INSTALL_BIN_PATH=bin
export GO_SOURCE_EXCLUDES=( \
    "test" \
)

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
export GOOS
export GOARCH
export CGO_ENABLED=0
