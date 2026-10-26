#!/bin/sh
# Builds Splitwiz on this computer and replaces the running container on the
# NAS. The database in pb_data is kept and migrations run on start.
#
#   scripts/deploy-to-nas.sh user@nas-address
#
# Copies into /volume1/docker/splitwiz (Synology's usual place); set
# SPLITWIZ_DIR to use another folder on the NAS. If that folder has no .env
# yet, one is made with this computer's time zone.
#
# Asks for the NAS password twice: once to copy, once for sudo.
set -e
TARGET=${1:?usage: $0 user@nas-address}
DIR=${SPLITWIZ_DIR:-/volume1/docker/splitwiz}
IMAGE=ghcr.io/selfhostit/splitwiz:latest

tz=${TZ:-$(cat /etc/timezone 2>/dev/null || timedatectl show -p Timezone --value 2>/dev/null || readlink /etc/localtime | sed 's|.*zoneinfo/||')}

cd "$(dirname "$0")/.."
docker build -t "$IMAGE" .

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
docker save "$IMAGE" | gzip > "$tmp/splitwiz-image.tar.gz"
cp docker-compose.yml "$tmp/"

echo "Copying to $TARGET:$DIR"
tar -C "$tmp" -cf - splitwiz-image.tar.gz docker-compose.yml | ssh "$TARGET" "mkdir -p $DIR && tar -C $DIR -xf -"

# sudo on DSM doesn't search /usr/local/bin, so call docker by its full path
ssh -t "$TARGET" "cd $DIR && { [ -f .env ] || echo 'TZ=${tz:-UTC}' > .env; } \
  && D=\$(command -v docker || echo /usr/local/bin/docker) \
  && sudo \$D load -i splitwiz-image.tar.gz \
  && sudo \$D compose up -d --force-recreate \
  && rm splitwiz-image.tar.gz && sleep 5 && sudo \$D exec splitwiz wget -qO- http://127.0.0.1:8090/api/health && echo"
