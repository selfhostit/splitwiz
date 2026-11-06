#!/bin/sh
# End-to-end check of docker-compose.yml: starts splitwiz from an image in a scratch folder,
# with a data folder that doesn't belong to the app user (like one an older, root-run
# version left behind), and checks that it takes the folder over and runs as that user
# on a read-only filesystem.
#
#   docker build -t ghcr.io/selfhostit/splitwiz:smoke . && SPLITWIZ_VERSION=smoke scripts/smoke-test.sh
set -eu

here=$(cd "$(dirname "$0")/.." && pwd)
dir=$(mktemp -d)
port=${PORT:-18090}
url=http://127.0.0.1:$port

cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then (cd "$dir" && docker compose logs) || true; fi
	(cd "$dir" && docker compose down -v) >/dev/null 2>&1 || true
	# the app owns pb_data now, so let a container remove it
	docker run --rm -v "$dir:/d" alpine:3.24 rm -rf /d/pb_data >/dev/null 2>&1 || true
	rm -r "$dir" 2>/dev/null || true
	exit "$status"
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
step() { echo "--- $*"; }

cp "$here/docker-compose.yml" "$dir/"
cat >"$dir/.env" <<ENV
SPLITWIZ_VERSION=${SPLITWIZ_VERSION:-1}
SPLITWIZ_PORT=$port
ENV
mkdir "$dir/pb_data"
echo "left by an older version" >"$dir/pb_data/old.txt"
cd "$dir"

healthy() {
	for _ in $(seq 1 30); do curl -sf "$url/api/health" >/dev/null && return 0; sleep 1; done
	return 1
}

step "Starting it"
docker compose up -d
healthy || fail "it did not become healthy"

step "Checking who it runs as"
id=$(docker compose exec -T splitwiz stat -c %u:%g /proc/1)
[ "$id" = 1000:1000 ] || fail "it runs as $id, not 1000:1000"
docker compose logs | grep -q "Running as user 1000:1000" || fail "no log line about the user"
for f in pb_data pb_data/data.db pb_data/old.txt; do
	[ "$(stat -c %u:%g "$f")" = 1000:1000 ] || fail "$f belongs to $(stat -c %u:%g "$f")"
done

step "Checking that only the data folder and /tmp can be written"
if docker compose exec -T splitwiz touch /app/x 2>/dev/null; then fail "/app is writable"; fi
docker compose exec -T splitwiz touch /tmp/x || fail "/tmp is not writable"

step "Restarting it"
docker compose restart
healthy || fail "it did not come back after a restart"

echo "OK"
