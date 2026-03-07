#!/usr/bin/env sh

WD="$PWD"
STORAGE_DIR="$WD/storage"
export ALLOY_FORGE_DIR="$WD/plugins"
export ALLOY_FORGE_MODCACHE="$STORAGE_DIR/storage/forge"

go run -C ../../collector/ . run \
  --storage.path="$WD/storage" \
  --stability.level="experimental" \
  "$WD/config.alloy"
