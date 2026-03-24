#!/bin/sh
set -eu

DB_HOST="postgresql"
DB_PORT="5432"
DB_USER="temporal"
DB_PASS="temporal"

# PostgreSQL readiness is guaranteed by the service_healthy depends_on
# condition in docker-compose.yml (pg_isready passes before this runs).
echo "PostgreSQL is ready — running schema migrations."

# ── Temporal persistence schema ───────────────────────────────────────────────
echo "Creating temporal database..."
temporal-sql-tool \
  --ep "$DB_HOST" --port "$DB_PORT" \
  -u "$DB_USER" -pw "$DB_PASS" \
  --pl postgres12 --db temporal \
  create

echo "Setting up temporal schema..."
temporal-sql-tool \
  --ep "$DB_HOST" --port "$DB_PORT" \
  -u "$DB_USER" -pw "$DB_PASS" \
  --pl postgres12 --db temporal \
  setup-schema -v 0.0

echo "Applying temporal schema migrations..."
temporal-sql-tool \
  --ep "$DB_HOST" --port "$DB_PORT" \
  -u "$DB_USER" -pw "$DB_PASS" \
  --pl postgres12 --db temporal \
  update-schema -d /etc/temporal/schema/postgresql/v12/temporal/versioned/

# ── Temporal visibility schema ────────────────────────────────────────────────
echo "Creating temporal_visibility database..."
temporal-sql-tool \
  --ep "$DB_HOST" --port "$DB_PORT" \
  -u "$DB_USER" -pw "$DB_PASS" \
  --pl postgres12 --db temporal_visibility \
  create

echo "Setting up temporal_visibility schema..."
temporal-sql-tool \
  --ep "$DB_HOST" --port "$DB_PORT" \
  -u "$DB_USER" -pw "$DB_PASS" \
  --pl postgres12 --db temporal_visibility \
  setup-schema -v 0.0

echo "Applying temporal_visibility schema migrations..."
temporal-sql-tool \
  --ep "$DB_HOST" --port "$DB_PORT" \
  -u "$DB_USER" -pw "$DB_PASS" \
  --pl postgres12 --db temporal_visibility \
  update-schema -d /etc/temporal/schema/postgresql/v12/visibility/versioned/

echo "Schema setup complete."
