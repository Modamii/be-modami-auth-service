#!/usr/bin/env bash
# register-connector.sh — registers the Debezium user_entity CDC connector.
# Usage: ./register-connector.sh [connect-url]
# Default connect URL: http://localhost:8083

set -euo pipefail

CONNECT_URL="${1:-http://localhost:8083}"
CONNECTOR_FILE="$(dirname "$0")/user-connector.json"
MAX_RETRIES=30
RETRY_INTERVAL=5

echo "Waiting for Kafka Connect at ${CONNECT_URL}..."
for i in $(seq 1 "$MAX_RETRIES"); do
  if curl -sf "${CONNECT_URL}/connectors" > /dev/null 2>&1; then
    echo "Kafka Connect is ready."
    break
  fi
  if [ "$i" -eq "$MAX_RETRIES" ]; then
    echo "ERROR: Kafka Connect did not become ready after $((MAX_RETRIES * RETRY_INTERVAL))s." >&2
    exit 1
  fi
  echo "  attempt $i/$MAX_RETRIES — retrying in ${RETRY_INTERVAL}s..."
  sleep "$RETRY_INTERVAL"
done

echo "Registering connector from ${CONNECTOR_FILE}..."
HTTP_CODE=$(curl -s -o /tmp/connect_response.json -w "%{http_code}" \
  -X POST "${CONNECT_URL}/connectors" \
  -H "Content-Type: application/json" \
  --data-binary "@${CONNECTOR_FILE}")

case "$HTTP_CODE" in
  201)
    echo "Connector registered successfully (HTTP 201)."
    ;;
  409)
    echo "Connector already exists (HTTP 409) — skipping."
    ;;
  *)
    echo "ERROR: unexpected response HTTP ${HTTP_CODE}:" >&2
    cat /tmp/connect_response.json >&2
    exit 1
    ;;
esac
