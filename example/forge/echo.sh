#!/usr/bin/env bash
# Send test Cloudflare LogPush payloads to the Forge cloudflare receiver.
#
# Usage:
#   ./example/forge/echo.sh

set -euo pipefail

ENDPOINT="http://localhost:43210"

echo "=== Cloudflare connectivity test ==="
curl -v -X POST "$ENDPOINT" -d 'test'
echo ""

echo ""
echo "=== Sending Cloudflare LogPush payload ==="
curl -v -X POST "$ENDPOINT" \
  -H "Content-Type: application/json" \
  -d '{"EdgeStartTimestamp":"2026-03-07T12:00:00Z","ClientIP":"1.2.3.4","ZoneName":"example.com","EdgeResponseStatus":200,"ClientRequestHost":"example.com","ClientRequestURI":"/hello"}'
echo ""
