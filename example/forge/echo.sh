#!/usr/bin/env bash
# Send test syslog messages to the Forge syslog receiver over UDP.
#
# Usage:
#   ./example/forge/echo.sh [count]
#
# Defaults to sending 5 messages to 127.0.0.1:43210.

set -euo pipefail

HOST="127.0.0.1"
PORT="43210"
COUNT="${1:-5}"

for i in $(seq 1 "$COUNT"); do
  ts=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
  msg="<86>1 ${ts} 127.0.0.1 forge-test $$ ID${i} - test syslog message ${i} of ${COUNT}"
  echo "$msg" | nc -u -w0 "$HOST" "$PORT"
  echo "sent: $msg"
done

echo "done — sent ${COUNT} syslog messages to ${HOST}:${PORT}"
