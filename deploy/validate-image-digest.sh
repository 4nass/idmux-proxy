#!/bin/sh

set -eu

digest=${1:-${IDMUX_IMAGE_DIGEST:-}}
prefix=sha256:

case "$digest" in
  "$prefix"*)
    hex=${digest#"$prefix"}
    ;;
  *)
    echo "IDMUX_IMAGE_DIGEST must be sha256 followed by 64 lowercase hex characters." >&2
    exit 2
    ;;
esac

if [ "${#hex}" -ne 64 ]; then
  echo "IDMUX_IMAGE_DIGEST must be sha256 followed by 64 lowercase hex characters." >&2
  exit 2
fi

case "$hex" in
  *[!0-9a-f]*)
    echo "IDMUX_IMAGE_DIGEST must be sha256 followed by 64 lowercase hex characters." >&2
    exit 2
    ;;
esac
