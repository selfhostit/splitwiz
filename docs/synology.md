# Running Splitwiz on a Synology NAS

This guide uses only the DSM 7 web interface and gives you Splitwiz at `https://<your-name>.synology.me`, with a free Let's Encrypt certificate. It was written on a DS220+; any Synology model that runs Container Manager (x86-64 or ARM) works the same way.

Comfortable with SSH? `curl -fsSL https://raw.githubusercontent.com/selfhostit/splitwiz/main/scripts/install.sh | sh` on the NAS does steps 1 to 3 for you (it installs into `/volume1/docker/splitwiz`).

Replace `<your-name>` with whatever hostname you pick in step 4.

## 1. Install Container Manager

Install **Container Manager** from Package Center if it is not there yet. It usually creates a `docker` shared folder.

## 2. Create the data folder

In File Station, create `docker/splitwiz/pb_data`. This is where the database and its backups live.

## 3. Create the project

Container Manager > Project > Create:

- Name: `splitwiz`
- Path: `/volume1/docker/splitwiz`
- Source: create a `docker-compose.yml` and paste in the [one from this repository](../docker-compose.yml). In the `TZ=${TZ:-UTC}` line, change `UTC` to your time zone, for example `TZ=${TZ:-Europe/Brussels}`.

Start the project. Splitwiz now runs on the NAS at port 8090, reachable only from the NAS itself.

## 4. Get an address and a certificate

Control Panel > External Access > DDNS > Add:

- Service provider: Synology
- Hostname: `<your-name>.synology.me`
- Tick **Get a certificate from Let's Encrypt**

## 5. Add a reverse proxy

Control Panel > Login Portal > Advanced > Reverse Proxy > Create:

- Source: HTTPS, hostname `<your-name>.synology.me`, port 443
- Destination: HTTP, `localhost`, port 8090

## 6. Use the certificate

Control Panel > Security > Certificate > Settings, and assign the `<your-name>.synology.me` certificate to the reverse-proxy entry you just made.

## 7. Open the router

Give the NAS a fixed address on your network (a DHCP reservation in the router), then forward TCP port 443 to it if it is not forwarded already.

Only want to use Splitwiz at home or over a VPN? Skip this step and the DDNS step, and use Tailscale or your VPN's address instead.

## 8. Add the two people

Open `https://<your-name>.synology.me`. On the setup screen, fill in a name, email and password for each of you. The first person can also sign in to the admin page at `/_/`.

## 9. Install it on the phones

Open `https://<your-name>.synology.me` on each phone, sign in, and choose **⋮ > Install app** in Chrome on Android or **Share > Add to Home Screen** (iOS/Safari).

## Updating

In Container Manager, open Image, select `ghcr.io/selfhostit/splitwiz` and choose **Update** when DSM offers it. Then Project > splitwiz > Action > **Build** to recreate the container. Your data stays in `pb_data` and migrations run on start.

Working on Splitwiz yourself? `scripts/deploy-to-nas.sh you@your-nas-address` builds the image on your computer, copies it to the NAS over SSH and recreates the container.

## Backups

Splitwiz keeps 7 nightly backups in `pb_data/backups`. To also keep a copy off the NAS, add `/volume1/docker/splitwiz` to a **Hyper Backup** task.
