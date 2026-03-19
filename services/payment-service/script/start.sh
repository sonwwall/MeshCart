#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../../.." && pwd)
cd "$ROOT_DIR"

export PAYMENT_SERVICE_CONFIG=${PAYMENT_SERVICE_CONFIG:-services/payment-service/config/payment-service.local.yaml}
exec go run ./services/payment-service/rpc
