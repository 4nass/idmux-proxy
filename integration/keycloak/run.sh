#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
fixture=integration/keycloak
binary_dir=$(mktemp -d)
# Use a separate Compose project for each run.
project="idmux-keycloak-${binary_dir##*.}"
project=${project,,}
cleanup() {
  docker compose -p "$project" -f "$fixture/compose.yml" down --timeout 5
  rm -f -- "$binary_dir/idmux-proxy"
  rmdir -- "$binary_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

go build -o "$binary_dir/idmux-proxy" ./cmd/idmux-proxy
docker compose -p "$project" -f "$fixture/compose.yml" up --detach
node "$fixture/oidc.mjs" "$binary_dir/idmux-proxy"
