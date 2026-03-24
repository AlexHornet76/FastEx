#!/usr/bin/env bash
set -euo pipefail

export GOTOOLCHAIN=local

BASE_URL="${1:-http://localhost:8080}"
USERNAME="${2:-test}"

go run ./scripts/auth_flow.go -base-url "$BASE_URL" -username "$USERNAME"