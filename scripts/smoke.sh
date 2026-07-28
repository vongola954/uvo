#!/usr/bin/env bash
set -e
BASE="${1:-http://127.0.0.1:8010}"
echo "== health =="
curl -sf "$BASE/health" | head -c 400
echo
echo "== metrics =="
curl -sf "$BASE/metrics"
echo
echo "== pages =="
for p in / /tracks.html /feed.html /playlists.html; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE$p")
  echo "$p $code"
done
echo "SMOKE OK"
