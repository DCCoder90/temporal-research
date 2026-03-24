#!/bin/sh
set -eu

FRONTEND="temporal-frontend:7233"
MAX_ATTEMPTS=60

echo "Waiting for Temporal frontend at ${FRONTEND}..."
attempt=0
until temporal --address "$FRONTEND" operator cluster health; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge "$MAX_ATTEMPTS" ]; then
    echo "ERROR: Temporal frontend did not become ready after ${MAX_ATTEMPTS} attempts."
    exit 1
  fi
  echo "  Not ready yet (attempt ${attempt}/${MAX_ATTEMPTS}), retrying in 5s..."
  sleep 5
done

echo "Temporal frontend is ready."

echo "Creating 'default' namespace..."
temporal --address "$FRONTEND" operator namespace create \
  --retention 3d \
  default 2>/dev/null \
  || echo "Namespace 'default' already exists — skipping."

echo "Namespace setup complete."
