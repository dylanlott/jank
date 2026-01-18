#!/usr/bin/env bash
set -euo pipefail

backup_dir="${BACKUP_DIR:-./backups}"
mkdir -p "$backup_dir"

driver="${JANK_DB_DRIVER:-postgres}"
driver="${driver,,}"

dsn="${JANK_DB_DSN:-}"
if [[ -z "$dsn" ]]; then
  dsn="${DATABASE_URL:-}"
fi

timestamp="$(date +%Y%m%d_%H%M%S)"

case "$driver" in
  sqlite|sqlite3)
    if [[ -z "$dsn" ]]; then
      dsn="./sqlite.db"
    fi
    db_path="$dsn"
    if [[ "$db_path" == file:* ]]; then
      db_path="${db_path#file:}"
      db_path="${db_path%%\?*}"
    fi
    if [[ ! -f "$db_path" ]]; then
      echo "sqlite db not found: $db_path" >&2
      exit 1
    fi
    out="$backup_dir/sqlite_backup_${timestamp}.db"
    cp "$db_path" "$out"
    echo "sqlite backup written to $out"
    ;;
  postgres|postgresql|pgx)
    if [[ -z "$dsn" ]]; then
      echo "JANK_DB_DSN or DATABASE_URL is required for postgres backups" >&2
      exit 1
    fi
    if ! command -v pg_dump >/dev/null 2>&1; then
      echo "pg_dump not found in PATH" >&2
      exit 1
    fi
    out="$backup_dir/postgres_backup_${timestamp}.sql"
    pg_dump "$dsn" > "$out"
    echo "postgres backup written to $out"
    ;;
  *)
    echo "unsupported JANK_DB_DRIVER: $driver" >&2
    exit 1
    ;;
 esac
