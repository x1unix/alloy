#!/usr/bin/env sh

WD="$PWD"
STORAGE_DIR="$WD/storage"
COLLECTOR_DIR="$(cd ../../collector && pwd)"

export ALLOY_FORGE_DIR="$WD/plugins"
export ALLOY_FORGE_MODCACHE="$STORAGE_DIR/forge"

go run -C "$COLLECTOR_DIR" . run \
  --storage.path="$WD/storage" \
  --stability.level="experimental" \
  --feature.community-components.enabled \
  "$WD/config.alloy"
