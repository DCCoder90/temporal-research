#!/bin/sh
# One-shot DB schema initialisation run by the temporal-setup container.
# Exits 0 on success. All temporal service containers depend on this completing.
set -eu

DB_PORT="${DB_PORT:-5432}"

echo "=== Setting up temporal database ==="
temporal-sql-tool \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --plugin postgres12 \
  --db temporal \
  create-database

temporal-sql-tool \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --plugin postgres12 \
  --db temporal \
  setup-schema -v 0.0

temporal-sql-tool \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --plugin postgres12 \
  --db temporal \
  update-schema -d /etc/temporal/schema/postgresql/v12/temporal/versioned

echo "=== Setting up temporal_visibility database ==="
temporal-sql-tool \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --plugin postgres12 \
  --db temporal_visibility \
  create-database

temporal-sql-tool \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --plugin postgres12 \
  --db temporal_visibility \
  setup-schema -v 0.0

temporal-sql-tool \
  --ep "${POSTGRES_SEEDS}" \
  --port "${DB_PORT}" \
  --user "${POSTGRES_USER}" \
  --password "${POSTGRES_PWD}" \
  --plugin postgres12 \
  --db temporal_visibility \
  update-schema -d /etc/temporal/schema/postgresql/v12/visibility/versioned

echo "=== Schema setup complete ==="
