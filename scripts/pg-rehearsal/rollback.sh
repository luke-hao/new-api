#!/bin/sh
set -eu
cd /opt/new-api-src/current
exec python3 scripts/pg-rehearsal/ops.py rollback
