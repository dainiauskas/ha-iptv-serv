# Vietinis IPTV tarpinis serveris (Hass.io Add-on)

Šis įskiepis parsisiunčia nurodytus `.m3u` grojaraščius (pvz., iš `iptv-org`), patikrina ar kanalai veikia, ir teikia **bendrą** sujungtą grojaraštį bei **atskirus** grojaraščius kiekvienam šaltiniui – juos galite naudoti savo įrenginiuose (pvz., Apple TV).

## Diegimas iš GitHub

1. **Pridėkite repozitoriją** į Home Assistant: **Settings** → **Add-ons** → **Add-on store** → viršuje dešinėje **⋮** → **Repositories**.
2. Įrašykite repozitorijos URL, pvz. `https://github.com/JŪSŲ_VARTOTOJAS/iptv-srv`.
3. Paspauskite **Check for updates**. Skiltyje **Add-on store** turėtų pasirodyti **IPTV Server add-on repository** ir įskiepis **IPTV Server**.
4. Atidarykite **IPTV Server** → **Install** (įdiegti). Pirmas diegimas gali užtrukti kelias minutes.

**Pastaba:** naudokite **viešą** repozitoriją – Home Assistant negali tiesiogiai pasiekti private GitHub repo.

**Forkinant repozitoriją:** faile `repository.yaml` pakeiskite `USERNAME` į savo GitHub vartotojo vardą ir `maintainer` į savo kontaktus.

## Nustatymai ir paleidimas

1. Įdiegus eikite į **Configuration** (Konfigūracija). **Playlists** – kiekvienam grojaraščiui nurodykite **name** (pavadinimą) ir **url**. Pavadinimas naudojamas adrese: `/playlist/[pavadinimas].m3u`. Numatyta: `lit`, `rus` (iptv-org kanalai).
2. Išsaugokite (**Save**). Rekomenduojama: **Start on boot**, **Watchdog**.
3. Paspauskite **START** (Paleisti).

## Naudojimas

IPTV grojaraštį nurodykite savo programoje (Apple TV, VLC ir kt.):

- **Bendras grojaraštis** (visi šaltiniai):  
  `http://<JŪSŲ_HA_IP>:8080/playlist.m3u`
- **Atskiri grojaraščiai** (pagal pavadinimą):  
  `http://<JŪSŲ_HA_IP>:8080/playlist/lit.m3u`, `.../playlist/rus.m3u` ir t. t. Taip pat veikia pagal indeksą: `.../playlist/0.m3u`, `.../playlist/1.m3u`.

Pvz.: `http://192.168.1.100:8080/playlist.m3u`
