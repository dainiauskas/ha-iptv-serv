# Local IPTV proxy server (Hass.io Add-on)

This add-on fetches the configured `.m3u` playlists (e.g. from `iptv-org`), checks that channels are reachable, and serves a **combined** playlist plus **per-source** playlists for use on your devices (e.g. Apple TV).

## Installation from GitHub

1. **Add the repository** in Home Assistant: **Settings** → **Add-ons** → **Add-on store** → top-right **⋮** → **Repositories**.
2. Enter the repository URL, e.g. `https://github.com/YOUR_USERNAME/iptv-srv`.
3. Tap **Check for updates**. In **Add-on store** you should see **IPTV Server add-on repository** and the **IPTV Server** add-on.
4. Open **IPTV Server** → **Install**. First install may take a few minutes.

**When forking:** In `repository.yaml` replace `USERNAME` with your GitHub username and `maintainer` with your contact details.

## Configuration and startup

1. After install go to **Configuration**. Under **Playlists** set **name** and **url** for each playlist. The name is used in the URL: `/playlist/[name].m3u`. Defaults: `lit`, `rus` (iptv-org).
2. **Save**. Recommended: **Start on boot**, **Watchdog**.
3. Tap **START**.

## Usage

Use these URLs in your IPTV app (Apple TV, VLC, etc.):

- **Combined playlist** (all sources):  
  `http://<YOUR_HA_IP>:8080/playlist.m3u`
- **Per-playlist** (by name):  
  `http://<YOUR_HA_IP>:8080/playlist/lit.m3u`, `.../playlist/rus.m3u`, etc. Index also works: `.../playlist/0.m3u`, `.../playlist/1.m3u`.

Example: `http://192.168.1.100:8080/playlist.m3u`
