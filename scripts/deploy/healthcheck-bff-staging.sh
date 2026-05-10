#!/bin/sh
# healthcheck-bff-staging.sh
# Polls the staging BFF /healthz endpoint (port 8081) until it responds 200
# or the retry limit is reached. Runs ON the EC2 instance via SSM RunShellScript.

set -e

RETRIES=10
SLEEP=5

i=0
while [ "$i" -lt "$RETRIES" ]; do
  if curl -sf http://127.0.0.1:8081/healthz > /dev/null; then
    echo "staging healthz OK"
    exit 0
  fi
  i=$((i + 1))
  echo "[$i/$RETRIES] staging healthz not ready, retrying in ${SLEEP}s..."
  sleep "$SLEEP"
done

echo "ERROR: staging healthz did not respond after $((RETRIES * SLEEP))s" >&2
exit 1
