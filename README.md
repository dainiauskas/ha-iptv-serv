# Local IPTV proxy server (Hass.io Add-on)

This add-on fetches the configured `.m3u` playlists (e.g. from `iptv-org`), checks that channels are reachable, and serves a **combined** playlist plus **per-source** playlists for use on your devices (e.g. Apple TV).

## Installation from GitHub

1. **Add the repository** in Home Assistant: **Settings** → **Add-ons** → **Add-on store** → top-right **⋮** → **Repositories**.
2. Enter the repository URL, e.g. `https://github.com/YOUR_USERNAME/iptv-srv`.
3. Tap **Check for updates**. In **Add-on store** you should see **IPTV Server add-on repository** and the **IPTV Server** add-on.
4. Open **IPTV Server** → **Install**. First install may take a few minutes.

**When forking:** In `repository.yaml` replace `USERNAME` with your GitHub username and `maintainer` with your contact details.

## Configuration and startup

1. After install go to **Configuration**.
   - **Playlists**: set **name** and **url** for each playlist. The name is used in the URL: `/playlist/[name].m3u`. Defaults: `lit`, `rus` (iptv-org).
   - **EPG URL** (optional): full URL to an **XMLTV** file. Programmes are matched to channels by **tvg-id** (from the M3U `#EXTINF` line). Leave empty for no EPG. Example: `https://example.com/epg.xml`. Cached for 6 hours.
2. **Save**. Recommended: **Start on boot**, **Watchdog**.
3. Tap **START**.

## Usage

### M3U playlists

Use these URLs in your IPTV app (Apple TV, VLC, etc.):

- **Combined playlist** (all sources):  
  `http://<YOUR_HA_IP>:8080/playlist.m3u`
- **Per-playlist** (by name):  
  `http://<YOUR_HA_IP>:8080/playlist/lit.m3u`, `.../playlist/rus.m3u`, etc. Index also works: `.../playlist/0.m3u`, `.../playlist/1.m3u`.

Example: `http://192.168.1.100:8080/playlist.m3u`

### Xtream Codes API (local network, no auth)

Apps that support Xtream (e.g. Tivimate, IPTVX) can use the same server as an Xtream source:

- **URL:** `http://<YOUR_HA_IP>:8080`
- **Username:** any (e.g. `local`) or leave empty
- **Password:** any (e.g. `local`) or leave empty

The add-on exposes `player_api.php`, `xmltv.php` (EPG), and stream URLs (`/live/...`, `get.php`). No real authentication; for local use only. If **EPG URL** is set in config, `xmltv.php` returns programme data so apps can show the TV guide.
