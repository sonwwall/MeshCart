#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

export APP_ENV="${APP_ENV:-dev}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export OTEL_EXPORTER_OTLP_ENDPOINT="${OTEL_EXPORTER_OTLP_ENDPOINT:-localhost:4319}"
export GATEWAY_ADDR="${GATEWAY_ADDR:-:8080}"
export USER_RPC_SERVICE="${USER_RPC_SERVICE:-UserService}"
export USER_RPC_ADDR="${USER_RPC_ADDR:-127.0.0.1:8888}"

echo "[INFO] starting gateway"
echo "[INFO] APP_ENV=$APP_ENV LOG_LEVEL=$LOG_LEVEL OTEL_EXPORTER_OTLP_ENDPOINT=$OTEL_EXPORTER_OTLP_ENDPOINT"
echo "[INFO] GATEWAY_ADDR=$GATEWAY_ADDR USER_RPC_SERVICE=$USER_RPC_SERVICE USER_RPC_ADDR=$USER_RPC_ADDR"

exec go run ./gateway/cmd/gateway
