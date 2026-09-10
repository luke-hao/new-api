#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

TAG="${1:-}"
case "$TAG" in
  latest|previous) ;;
  *) printf 'Usage: %s latest|previous\n' "$0" >&2; exit 2 ;;
esac

ENV_FILE=/etc/restic/r2.env
PASSWORD_FILE=/etc/restic/password
HOST_NAME=ns1009363
BASE=/var/lib/ns1009363-r2-backup
STAGING=/var/lib/ns1009363-r2-backup/staging
LOCK=/run/lock/ns1009363-r2-backup.lock
SOURCE_DB=/opt/new-api/data/one-api.db
TARGET_DB="$STAGING/newapi/one-api.db"

test -r "$ENV_FILE"
test -r "$PASSWORD_FILE"
exec 9>"$LOCK"
flock -w 1800 9
DB_ENGINE=$(docker inspect new-api | python3 -c 'import json,sys; o=json.load(sys.stdin)[0]; e=dict(x.split("=",1) for x in o["Config"]["Env"] if "=" in x); d=e.get("SQL_DSN",""); print("postgres" if d.startswith(("postgres://","postgresql://")) else "sqlite" if not d or d.startswith("sqlite") else "unsupported")')
case "$DB_ENGINE" in
  sqlite) test -r "$SOURCE_DB" ;;
  postgres) TARGET_DB="$STAGING/newapi/new-api.pgdump" ;;
  *) printf 'Unsupported NewAPI database engine\n' >&2; exit 6 ;;
esac


set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
export RESTIC_PASSWORD_FILE="$PASSWORD_FILE"

install -d -m 700 "$BASE"

clear_staging() {
  if [[ -e "$STAGING" ]]; then
    local resolved
    resolved="$(readlink -f -- "$STAGING")"
    if [[ "$resolved" != '/var/lib/ns1009363-r2-backup/staging' ]]; then
      printf 'Refusing unsafe staging cleanup: %s\n' "$resolved" >&2
      exit 91
    fi
    rm -rf -- "$resolved"
  fi
}

clear_staging
trap clear_staging EXIT

install -d -m 700 \
  "$STAGING/newapi" \
  "$STAGING/cpamc/auths" \
  "$STAGING/reverse-proxy/certs"

if [[ "$DB_ENGINE" == 'postgres' ]]; then
  docker exec new-api-pg pg_dump -U postgres -d newapi     --format=custom --no-owner --no-acl --exclude-table-data=public.logs > "$TARGET_DB"
  test -s "$TARGET_DB"
  docker exec -i new-api-pg pg_restore --list < "$TARGET_DB" > /dev/null
  backup_log_rows=0
  install -m 600 /opt/new-api/postgres-client.env "$STAGING/newapi/postgres-client.env"
  install -m 600 /opt/new-api/postgres-server.env "$STAGING/newapi/postgres-server.env"
else
# Build a compact, transactionally consistent SQLite snapshot. All schemas are
# retained, but rows from the high-volume logs table are intentionally omitted.
SCHEMA_SQL="$STAGING/newapi/schema.sql"
INDEX_SQL="$STAGING/newapi/indexes.sql"
COPY_SQL="$STAGING/newapi/copy.sql"
COPY_COUNTS="$STAGING/newapi/copy-counts.txt"

sqlite3 -readonly -cmd '.timeout 60000' "$SOURCE_DB" \
  "SELECT sql || ';' FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND sql IS NOT NULL ORDER BY name;" \
  > "$SCHEMA_SQL"

sqlite3 -readonly -cmd '.timeout 60000' "$SOURCE_DB" \
  "SELECT sql || ';' FROM sqlite_master WHERE type='index' AND sql IS NOT NULL ORDER BY name;" \
  > "$INDEX_SQL"

mapfile -t CORE_TABLES < <(
  sqlite3 -readonly -cmd '.timeout 60000' "$SOURCE_DB" \
    "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name <> 'logs' ORDER BY name;"
)

sqlite3 "$TARGET_DB" < "$SCHEMA_SQL"

{
  printf '.timeout 60000\n'
  printf "ATTACH DATABASE '%s' AS src;\n" "$SOURCE_DB"
  printf 'PRAGMA foreign_keys=OFF;\n'
  printf 'BEGIN;\n'
  for table in "${CORE_TABLES[@]}"; do
    printf 'INSERT INTO "%s" SELECT * FROM src."%s";\n' "$table" "$table"
  done
  for table in "${CORE_TABLES[@]}"; do
    printf "SELECT '%s|' || (SELECT count(*) FROM src.\"%s\") || '|' || (SELECT count(*) FROM main.\"%s\");\n" \
      "$table" "$table" "$table"
  done
  printf 'COMMIT;\n'
  printf 'DETACH DATABASE src;\n'
} > "$COPY_SQL"

sqlite3 "$TARGET_DB" < "$COPY_SQL" > "$COPY_COUNTS"

if ! awk -F '|' '
  NF != 3 || $2 != $3 {
    printf "Backup row-count mismatch: %s\n", $0 > "/dev/stderr"
    bad = 1
  }
  END { exit bad }
' "$COPY_COUNTS"; then
  exit 4
fi

sqlite3 "$TARGET_DB" < "$INDEX_SQL"

db_check="$(sqlite3 -readonly "$TARGET_DB" 'PRAGMA quick_check;')"
if [[ "$db_check" != 'ok' ]]; then
  printf 'SQLite core snapshot quick_check failed: %s\n' "$db_check" >&2
  exit 3
fi

backup_log_rows="$(sqlite3 -readonly "$TARGET_DB" 'SELECT count(*) FROM logs;')"
if [[ "$backup_log_rows" != '0' ]]; then
  printf 'Expected an empty logs table in core snapshot, found %s rows\n' "$backup_log_rows" >&2
  exit 5
fi

rm -f -- "$SCHEMA_SQL" "$INDEX_SQL" "$COPY_SQL" "$COPY_COUNTS"

fi

install -m 600 /opt/new-api/.env "$STAGING/newapi/.env"
install -m 644 /opt/new-api/docker-compose.yml "$STAGING/newapi/docker-compose.yml"

install -m 600 /opt/cli-proxy-api/config.yaml "$STAGING/cpamc/config.yaml"
install -m 644 /opt/cli-proxy-api/docker-compose.yml "$STAGING/cpamc/docker-compose.yml"
install -m 644 /opt/cli-proxy-api/management.html "$STAGING/cpamc/management.html"
cp -a /opt/cli-proxy-api/auths/. "$STAGING/cpamc/auths/"

install -m 644 /opt/reverse-proxy/Caddyfile "$STAGING/reverse-proxy/Caddyfile"
install -m 644 /opt/reverse-proxy/cloudflare-ips.txt "$STAGING/reverse-proxy/cloudflare-ips.txt"
install -m 644 /opt/reverse-proxy/docker-compose.yml "$STAGING/reverse-proxy/docker-compose.yml"
cp -a /opt/reverse-proxy/certs/. "$STAGING/reverse-proxy/certs/"

{
  printf 'created_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'source_host=%s\n' "$HOST_NAME"
  printf 'snapshot_role=%s\n' "$TAG"
  printf 'newapi_database_engine=%s\n' "$DB_ENGINE"
  printf 'newapi_database_scope=all_schema_and_all_data_except_logs_rows\n'
  printf 'newapi_logs_rows_in_backup=0\n'
  printf 'newapi_core_database_bytes=%s\n' "$(stat -c '%s' "$TARGET_DB")"
  printf 'newapi_image=%s\n' "$(docker inspect --format '{{.Config.Image}}' new-api)"
  printf 'cpamc_image=%s\n' "$(docker inspect --format '{{.Config.Image}}' cli-proxy-api)"
} > "$STAGING/backup-info.txt"

find "$STAGING" -type f -print0 \
  | sort -z \
  | xargs -0 sha256sum > "$STAGING/SHA256SUMS"

if [[ "${BACKUP_DRY_RUN:-0}" == '1' ]]; then
  printf 'Dry run successful: core database size=%s bytes, logs rows=%s\n' \
    "$(stat -c '%s' "$TARGET_DB")" "$backup_log_rows"
  exit 0
fi

restic backup "$STAGING" \
  --host "$HOST_NAME" \
  --tag "$TAG"

restic forget \
  --host "$HOST_NAME" \
  --tag "$TAG" \
  --keep-last 1 \
  --group-by host,tags

logger -t ns1009363-r2-backup "Completed encrypted R2 core backup role=$TAG logs=excluded"
