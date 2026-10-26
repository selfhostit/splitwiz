#!/bin/sh
# Installs or updates Splitwiz with Docker on this machine: a Linux server,
# VPS, Raspberry Pi, or a NAS with SSH and Docker (Synology, QNAP, Unraid,
# TrueNAS SCALE, OpenMediaVault, ...).
#
#   curl -fsSL https://raw.githubusercontent.com/selfhostit/splitwiz/main/scripts/install.sh | sh
#
# It asks a few questions (Enter keeps the default), writes docker-compose.yml
# and .env into the install folder, and starts Splitwiz. Run it again to
# update; your answers in .env are kept.
#
# Without a terminal, or with SPLITWIZ_YES=1, it uses the defaults and these
# variables if set: SPLITWIZ_DIR, TZ, SPLITWIZ_LISTEN, SPLITWIZ_PORT.
set -e

REPO_RAW=https://raw.githubusercontent.com/selfhostit/splitwiz/main
IMAGE=ghcr.io/selfhostit/splitwiz

# set when run as scripts/install.sh from a clone rather than through a pipe
case $0 in
	*install.sh) CHECKOUT=$(cd "$(dirname "$0")/.." && pwd) ;;
esac
[ -n "$CHECKOUT" ] && [ ! -f "$CHECKOUT/docker-compose.yml" ] && CHECKOUT=

say() { printf '%s\n' "$*"; }
fail() { say "Error: $*" >&2; exit 1; }

# Read answers from the terminal even when the script itself comes from a pipe.
if [ -z "$SPLITWIZ_YES" ] && [ -r /dev/tty ] && (: < /dev/tty) 2>/dev/null; then
	INTERACTIVE=1
fi

# ask "Question" default -> prints the answer
ask() {
	if [ -n "$INTERACTIVE" ]; then
		printf '%s [%s]: ' "$1" "$2" > /dev/tty
		read -r answer < /dev/tty || answer=
		printf '%s\n' "${answer:-$2}"
	else
		printf '%s\n' "$2"
	fi
}

# --- Docker ---------------------------------------------------------------

DOCKER=$(command -v docker || true)
# Synology puts docker here and keeps it off sudo's PATH
[ -z "$DOCKER" ] && [ -x /usr/local/bin/docker ] && DOCKER=/usr/local/bin/docker
[ -n "$DOCKER" ] || fail "Docker is not installed. Install it first (on a NAS, from its app store), then run this again."

if ! "$DOCKER" info > /dev/null 2>&1; then
	if command -v sudo > /dev/null 2>&1 && [ "$(id -u)" != 0 ]; then
		say "Docker needs administrator rights here, so you may be asked for your password."
		DOCKER="sudo $DOCKER"
		$DOCKER info > /dev/null 2>&1 || fail "Docker is installed but not running, or this account may not use it."
	else
		fail "Docker is installed but not running, or this account may not use it."
	fi
fi

if $DOCKER compose version > /dev/null 2>&1; then
	COMPOSE="$DOCKER compose"
elif command -v docker-compose > /dev/null 2>&1; then
	case $DOCKER in sudo*) COMPOSE="sudo docker-compose" ;; *) COMPOSE=docker-compose ;; esac
else
	COMPOSE=
fi

# --- Questions --------------------------------------------------------------

if [ -n "$SPLITWIZ_DIR" ]; then
	default_dir=$SPLITWIZ_DIR
elif [ -d /volume1/docker ]; then
	default_dir=/volume1/docker/splitwiz # Synology
elif [ -d /mnt/user/appdata ]; then
	default_dir=/mnt/user/appdata/splitwiz # Unraid
else
	default_dir=$PWD/splitwiz
fi

say ""
say "Splitwiz installer"
say ""
DIR=$(ask "Install folder (the database is kept here)" "$default_dir")
mkdir -p "$DIR" || fail "Could not create $DIR. Pick another folder, or run the installer with sudo."
cd "$DIR"

if [ -f .env ]; then
	say "Found settings from an earlier install in $DIR/.env; keeping them."
	# shellcheck disable=SC1091
	. ./.env
else
	detected_tz=${TZ:-}
	[ -z "$detected_tz" ] && [ -r /etc/timezone ] && detected_tz=$(cat /etc/timezone)
	[ -z "$detected_tz" ] && command -v timedatectl > /dev/null 2>&1 &&
		detected_tz=$(timedatectl show -p Timezone --value 2> /dev/null || true)
	[ -z "$detected_tz" ] && [ -L /etc/localtime ] &&
		detected_tz=$(readlink /etc/localtime | sed 's|.*zoneinfo/||')
	TZ=$(ask "Time zone" "${detected_tz:-UTC}")

	say ""
	say "Who should be able to connect?"
	say "  1) Only this machine. Pick this if you will put a reverse proxy with HTTPS in front"
	say "     (the NAS's own reverse proxy, Caddy, Nginx Proxy Manager, Traefik, a tunnel)."
	say "  2) Every device on your network, over plain http://. Easiest to try out, but phones"
	say "     need HTTPS to install the app and get notifications."
	case ${SPLITWIZ_LISTEN:-127.0.0.1} in 0.0.0.0) d=2 ;; *) d=1 ;; esac
	case $(ask "Choose 1 or 2" "$d") in
		2) SPLITWIZ_LISTEN=0.0.0.0 ;;
		*) SPLITWIZ_LISTEN=127.0.0.1 ;;
	esac
	SPLITWIZ_PORT=$(ask "Port" "${SPLITWIZ_PORT:-8090}")

	cat > .env <<-EOF
		# Splitwiz settings; see https://github.com/selfhostit/splitwiz/blob/main/.env.example
		TZ=$TZ
		SPLITWIZ_LISTEN=$SPLITWIZ_LISTEN
		SPLITWIZ_PORT=$SPLITWIZ_PORT
	EOF
fi
SPLITWIZ_LISTEN=${SPLITWIZ_LISTEN:-127.0.0.1}
SPLITWIZ_PORT=${SPLITWIZ_PORT:-8090}
SPLITWIZ_DATA=${SPLITWIZ_DATA:-./pb_data}
mkdir -p "$SPLITWIZ_DATA" 2> /dev/null || true

# --- Start ------------------------------------------------------------------

# Use the compose file from this checkout when run from one, else download it.
if [ -n "$CHECKOUT" ]; then
	[ "$CHECKOUT" -ef . ] || cp "$CHECKOUT/docker-compose.yml" docker-compose.yml
elif command -v curl > /dev/null 2>&1; then
	curl -fsSL "$REPO_RAW/docker-compose.yml" -o docker-compose.yml
else
	wget -qO docker-compose.yml "$REPO_RAW/docker-compose.yml"
fi

say ""
say "Downloading and starting Splitwiz..."
if [ -n "$COMPOSE" ]; then
	$COMPOSE pull --quiet || say "Could not download the latest image; using the one already on this machine."
	$COMPOSE up -d
else
	# no Compose on this machine: the same container with plain docker run
	say "(Docker Compose not found, using docker run instead.)"
	$DOCKER pull "$IMAGE:${SPLITWIZ_VERSION:-latest}" ||
		say "Could not download the latest image; using the one already on this machine."
	$DOCKER rm -f splitwiz > /dev/null 2>&1 || true
	case $SPLITWIZ_DATA in /*) data=$SPLITWIZ_DATA ;; *) data=$PWD/${SPLITWIZ_DATA#./} ;; esac
	$DOCKER run -d --name splitwiz --restart always --memory 256m \
		-p "$SPLITWIZ_LISTEN:$SPLITWIZ_PORT:8090" -v "$data:/app/pb_data" \
		-e "TZ=$TZ" -e "SPLITWIZ_PUSH_CONTACT=${SPLITWIZ_PUSH_CONTACT:-}" \
		"$IMAGE:${SPLITWIZ_VERSION:-latest}" > /dev/null
fi

i=0
until $DOCKER exec splitwiz wget -qO- http://127.0.0.1:8090/api/health > /dev/null 2>&1; do
	i=$((i + 1))
	[ $i -ge 30 ] && fail "Splitwiz did not start. See what went wrong with: $DOCKER logs splitwiz"
	sleep 2
done

# --- Next steps -------------------------------------------------------------

lan_ip=$(hostname -I 2> /dev/null | awk '{print $1}')
[ -z "$lan_ip" ] && lan_ip=$(ip route get 1.1.1.1 2> /dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p')
[ -z "$lan_ip" ] && lan_ip="<this-machine's-address>"

say ""
say "Splitwiz is running, with its data in $DIR."
say ""
if [ "$SPLITWIZ_LISTEN" = 0.0.0.0 ]; then
	say "Open http://$lan_ip:$SPLITWIZ_PORT from any device on your network and add the two"
	say "people on the setup screen."
else
	say "Next, point your reverse proxy at http://127.0.0.1:$SPLITWIZ_PORT, open its HTTPS"
	say "address and add the two people on the setup screen."
	say "To try it first from another computer:  ssh -L $SPLITWIZ_PORT:127.0.0.1:$SPLITWIZ_PORT $(id -un)@$lan_ip"
	say "and then open http://localhost:$SPLITWIZ_PORT"
fi
say ""
say "Phones need an HTTPS address to install the app and get notifications:"
say "https://github.com/selfhostit/splitwiz#using-it-from-your-phones"
say "To update later, run this installer again."
