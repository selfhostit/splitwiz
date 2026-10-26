<div align="center">

<img src="docs/logo.png" alt="Splitwiz, a grumpy little wizard holding back a river of euro bills with his staff" width="420">

# Splitwiz

*Meet Splitwiz, the wizard who splits your costs. He stands in the river of receipts and bills so the two of you don't have to. Let Splitwiz take care of the money; you take care of each other.*

**Split shared expenses with your partner or roommate, on a server you own.**

A small, self-hosted alternative to Splitwise for two people. No ads, no daily limits, no account on someone else's cloud. One Docker container, installable on your phone like a native app.

[![CI](https://github.com/selfhostit/splitwiz/actions/workflows/ci.yml/badge.svg)](https://github.com/selfhostit/splitwiz/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Docker image](https://img.shields.io/badge/docker-ghcr.io%2Fselfhostit%2Fsplitwiz-blue?logo=docker&logoColor=white)](https://github.com/selfhostit/splitwiz/pkgs/container/splitwiz)
[![Buy me a coffee](https://img.shields.io/badge/buy%20me%20a%20coffee-support-FFDD00?logo=buymeacoffee&logoColor=black)](https://buymeacoffee.com/kalfasyan)

<img src="docs/screenshots/hero.png" alt="Splitwiz on a phone: balance, adding an expense, stats, expense detail with a category chart, and settings in dark mode" width="100%">

</div>

## Why Splitwiz

- **Your data stays home.** Everything lives in one SQLite file on your own machine: a NAS, a home server, a Raspberry Pi-class box or a cheap VPS.
- **Fast to use.** Adding an expense is an amount, a description and one tap. The balance is the first thing you see.
- **Leaving Splitwise is easy.** Import a group's CSV export and you land on the same balance, with every expense and payment kept.
- **Tiny to run.** One container, about 30 MB of memory at rest, no external database.

## Features

- **Add an expense in a few taps**: amount, what it was for, who paid, and how to split it (50/50, percentage, exact amount, or "the other person owes all").
- **Quick add**: the expenses you add most often show up as one-tap chips, and typing a past description fills in who paid, the split and the category from last time.
- **One balance** on the home screen: who owes whom, and how much.
- **Settle up** with one tap, in full or in part.
- **History** grouped by month, with search and filters (type, who paid, category, amount). Export just what you filtered as CSV. Swipe a row left to edit or delete it.
- **Undo** instead of "Are you sure?": deleting anything shows an Undo bar for a few seconds.
- **Works offline**: the app opens with the last data loaded, and expenses, payments and comments added without a connection are saved on the phone and sync once the server is reachable.
- **Comments on expenses**: both of you can leave comments on any expense, like a small chat thread. A comment typed while adding an expense becomes the first one.
- **Push notifications** straight from your server, no account or extra app needed. Turn them on per phone under Settings > Notifications to hear when the other person adds or changes an expense or comments, when a repeating expense is added, and once a month when spending is heading over the budget. Tapping one opens that expense.
- **Repeating expenses** (weekly, monthly, yearly) are added automatically on their day, and caught up if the server was off. Any expense can be turned into a repeating one, and Settings suggests charges that already look monthly ("These look monthly"), for example right after an import.
- **Coming up** on the home screen lists the repeating expenses due in the next 30 days or 3 months, with their total, so nothing arrives as a surprise.
- **Monthly budget** shared by both of you. Stats shows this month's spending against it and where the month is heading: everyday spending is projected with a simple linear trend and the repeating expenses still due are added on their day.
- **Categories** are yours to manage under Settings: add, rename, reorder, change the emoji, or delete. Each one can have its **own monthly limit**; Stats shows what's left and you both get a notification at 90%.
- **Stats** for this month, last month, 3 or 12 months, this year or all time: total spent, spending per month, a breakdown by category (with who paid what in each) you can drill into, and the biggest expenses.
- **Category trend on every expense**: open an expense to see what you spent in its category over the last three months.
- **Import from Splitwise**: pick a group's CSV export under Settings. Importing the same file again only adds new rows.
- **Live sync** between both phones, **light and dark mode**, and a **CSV export** of everything.
- **Installable web app (PWA)** for Android and iOS, with its own home-screen icon. Long-press the icon for **Add expense** and **Settle up** shortcuts.

<table>
  <tr>
    <td><img src="docs/screenshots/home.png" alt="Home screen with the balance and recent expenses" width="220"></td>
    <td><img src="docs/screenshots/add-expense.png" alt="Adding an expense with split and repeat options" width="220"></td>
    <td><img src="docs/screenshots/stats.png" alt="Stats tab with spending per month and per category" width="220"></td>
    <td><img src="docs/screenshots/history.png" alt="History grouped by month with search" width="220"></td>
  </tr>
</table>

## Quick start

**Never heard of Docker?** Follow the [step-by-step guide for non-technical people](docs/easy-install.md): a Windows PC, Mac, Linux computer or Raspberry Pi at home, all by clicking and pasting.

Otherwise you need Docker. Pick whichever fits your machine; both end at the same place.

**With a terminal** (Linux server, VPS, Raspberry Pi, or a NAS with SSH):

```sh
curl -fsSL https://raw.githubusercontent.com/selfhostit/splitwiz/main/scripts/install.sh | sh
```

It asks for an install folder, your time zone and who may connect (Enter keeps each default), then starts Splitwiz and prints the address to open.

**Without a terminal** (a NAS's Docker app, Portainer, or any Compose tool): use [`docker-compose.yml`](docker-compose.yml) as it is, or with a `.env` next to it (see [`.env.example`](.env.example)). From a shell that is:

```sh
mkdir splitwiz && cd splitwiz
curl -O https://raw.githubusercontent.com/selfhostit/splitwiz/main/docker-compose.yml
docker compose up -d
```

Then open Splitwiz (`http://localhost:8090` on the same machine) and **add the two people on the setup screen**. The first person also becomes the admin for the admin page at `/_/`. That's the whole setup: there are no sign-ups, so these are the only two accounts.

The setup screen is open to whoever reaches it first, so finish it before you make Splitwiz reachable from the internet. By default it only listens on the machine itself.

<details>
<summary>Prefer plain <code>docker run</code>?</summary>

```sh
docker run -d --name splitwiz --restart always \
  -p 127.0.0.1:8090:8090 -v "$PWD/pb_data:/app/pb_data" -e TZ=Europe/Brussels \
  ghcr.io/selfhostit/splitwiz:latest
```

</details>

### Settings

All optional, in `.env` next to `docker-compose.yml` (the installer writes it for you):

| Setting | Default | What it does |
| --- | --- | --- |
| `TZ` | `UTC` | Your time zone, e.g. `Europe/Brussels`. Decides when repeating expenses are added and when the nightly backup runs. |
| `SPLITWIZ_LISTEN` | `127.0.0.1` | `127.0.0.1`: only this machine and a reverse proxy on it can connect. `0.0.0.0`: every device on your network can. |
| `SPLITWIZ_PORT` | `8090` | The port on this machine. |
| `SPLITWIZ_DATA` | `./pb_data` | Where the database, backups and push key live. |
| `SPLITWIZ_VERSION` | `latest` | Image tag; a commit SHA pins one build. |
| `SPLITWIZ_PUSH_CONTACT` | | Optional `mailto:` or `https:` address push services can contact. |

## Using it from your phones

Phones need to reach Splitwiz over **HTTPS** to install it as an app and to get notifications. Plain `http://` on your home network works in a browser, but not for those two. Any of these gives you HTTPS; point it at `http://127.0.0.1:8090`:

- **Your NAS's own reverse proxy** with its free certificate: Synology (see the [guide](docs/synology.md)), QNAP, or Unraid and TrueNAS with an Nginx Proxy Manager container.
- **[Caddy](https://caddyserver.com)** on a server with a domain name. This is the whole config:

  ```
  splitwiz.example.com {
      reverse_proxy 127.0.0.1:8090
  }
  ```

- **[Tailscale](https://tailscale.com)** if you'd rather not open your router: `tailscale serve --bg 8090` gives an HTTPS address that only your own devices can reach.
- **A Cloudflare Tunnel**, Traefik or Nginx Proxy Manager work just as well.

Then open the address on each phone, sign in, and choose **⋮ > Install app** in Chrome on Android or **Share > Add to Home Screen** (iOS/Safari).

### Step-by-step guides

- [Without technical knowledge](docs/easy-install.md): a Windows PC, Mac, Linux computer or Raspberry Pi at home, with Tailscale for phones.
- [Synology NAS (DSM 7, Container Manager)](docs/synology.md): container, free `*.synology.me` address, Let's Encrypt certificate and reverse proxy, all from the DSM interface.

### Where it runs

- **Should work** on anything that runs Docker containers for `linux/amd64`, `linux/arm64` or `linux/arm/v7`. That covers x86 and ARM servers and VPSes, Raspberry Pi 2 and newer (not the Pi Zero or Pi 1), and NAS systems with a Docker app: Synology (Container Manager), QNAP (Container Station), Unraid, TrueNAS SCALE, OpenMediaVault, Asustor and UGREEN, plus dashboards like Portainer and CasaOS that accept a Compose file.
- **Won't work** on NAS models without Docker support (many budget and older models), or hosts without containers at all. Splitwiz is not yet listed in NAS app stores, so on those you paste the Compose file yourself.

## Updating

Run the installer again, or:

```sh
docker compose pull && docker compose up -d
```

Every push to `main` publishes a new `latest` image. Your data and settings stay in `pb_data` and `.env`, and database migrations run on start.

## Backups

- Splitwiz makes a backup every evening at 20:00 (in your `TZ`) into `pb_data/backups` and keeps the last 7. You can change this in the admin page under Settings > Backups.
- Copy the `pb_data` folder somewhere else from time to time to keep a backup off the machine.
- Settings > Export all data (CSV) in the app gives you a human-readable copy.

## Good to know

- Splitwiz is built for **exactly two people** sharing costs, such as a couple or two roommates. It does not do groups.
- Amounts are shown in **euros** for now.
- **Notifications** need the app installed (on iPhone: added to the Home Screen, iOS 16.4 or newer) and the server able to reach the internet, since they are delivered through Apple's and Google's push services. The key that signs them is created on first use in `pb_data/push_keys.json`; if it is lost, turn notifications on again on each phone. In **Brave**, also turn on "Use Google services for push messaging" under `brave://settings/privacy` and restart Brave; it is off by default.

## How it is built

| Part | Choice |
| --- | --- |
| Backend | [PocketBase](https://pocketbase.io) used as a Go framework (`backend/`), SQLite database |
| Money logic | `backend/ledger`: integer cents, split, balance and repeat-date rules, unit tested |
| Splitwise import | `backend/splitwise`: parses the export and keeps each row's exact split |
| Web app | Svelte 5, Vite and Tailwind (`web/`), installable PWA |
| Packaging | One Docker image (`Dockerfile`) for x86-64 and ARM, published to `ghcr.io/selfhostit/splitwiz` by GitHub Actions |

The server computes what each person owes for every expense itself, so a phone can never store an inconsistent split.

## Development

```sh
# backend (needs Go 1.27+)
cd backend
go run . superuser upsert admin@example.com changeme123
go run . serve            # http://127.0.0.1:8090, admin at /_/

# web app with hot reload (needs Node 22+), in another terminal
cd web
npm install
npm run dev               # proxies /api to :8090
```

Tests: `cd backend && go test ./...` and `cd web && npm test && npm run check`.

`scripts/deploy-to-nas.sh user@host` builds the image on your computer and replaces the running container on a NAS over SSH (in `/volume1/docker/splitwiz`, or `SPLITWIZ_DIR`), for when you want to try a change before pushing it.

## Contributing

Issues and pull requests are welcome. Please run the tests above before opening a pull request; CI runs the same checks.

## Support

Splitwiz is free and stays free. If it saves the two of you some hassle, you can [buy me a coffee](https://buymeacoffee.com/kalfasyan) ☕

## License

[MIT](LICENSE)
