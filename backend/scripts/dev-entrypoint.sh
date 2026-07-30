#!/bin/sh
# Dev container entrypoint: wait for Postgres to be resolvable + accepting
# connections before starting Air, then exec Air so it becomes PID 1's child
# and receives signals cleanly.
#
# depends_on: service_healthy gates *container start*, but in-container DNS for
# the `db` service name can lag a beat behind; this loop closes that race.
set -e

DB_HOST="${DB_HOST:-db}"
DB_PORT="${DB_PORT:-5432}"

echo "dev-entrypoint: waiting for ${DB_HOST}:${DB_PORT}..."
i=0
until nc -z "$DB_HOST" "$DB_PORT" 2>/dev/null; do
  i=$((i + 1))
  if [ "$i" -ge 60 ]; then
    echo "dev-entrypoint: timed out waiting for ${DB_HOST}:${DB_PORT}" >&2
    exit 1
  fi
  sleep 1
done
echo "dev-entrypoint: ${DB_HOST}:${DB_PORT} is up — starting Air"

exec air -c .air.toml
