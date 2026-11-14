#!/usr/bin/env bash
# scripts/replay.sh — send a fixture payload to the local backend.
# Usage: make replay FILE=backend/testdata/payloads/sample.body
# The file must contain the raw form body (e.g. "j=<base64...>") exactly as the
# agent sends it; envelope format per docs/PROTOCOL.md section 2.
set -euo pipefail

FILE="${1:?usage: replay.sh <payload-file>}"
BASE_URL="${LUMA_REPLAY_URL:-http://127.0.0.1:8080}"

curl -sf -X POST "$BASE_URL/v2/" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-binary "@$FILE" \
  >/dev/null && echo "replayed $FILE -> $BASE_URL/v2/"
