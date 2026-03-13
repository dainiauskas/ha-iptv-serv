# Vietinis IPTV tarpinis serveris (Hass.io Add-on)

Šis įskiepis parsisiunčia nurodytus `.m3u` grojaraščius (pvz., iš `iptv-org`), patikrina ar kanalai veikia, ir teikia **bendrą** sujungtą grojaraštį bei **atskirus** grojaraščius kiekvienam šaltiniui – juos galite naudoti savo įrenginiuose (pvz., Apple TV).

## Kaip įdiegti į Home Assistant (Hass.io)

### 1 žingsnis: Prisijungimas prie Home Assistant failų sistemos
Tam, kad įkeltumėte šarvą (add-on), jums reikės prieigos prie Home Assistant `/addons` katalogo. Tam galite naudoti vieną iš šių populiarių oficialių įskiepių (jei dar neturite, įdiekite iš Add-on Store):
- **Samba share**
- **Advanced SSH & Web Terminal**
- **Studio Code Server** / **File editor**

### 2 žingsnis: Katalogo sukūrimas
Prieikite prie savo Home Assistant failų ir suraskite `addons` katalogą (šalia `config`, `share`, `backup` katalogų).
1. `addons` kataloge sukurkite naują aplanką pavadinimu `iptv-srv`.

### 3 žingsnis: Failų perkėlimas
Į ką tik sukurtą `addons/iptv-srv` katalogą perkelkite šiuos 6 projekto failus:
- `build.yaml`
- `config.yaml`
- `Dockerfile`
- `logo.png` *(add-on ikona Add-on Store)*
- `main.go`
- `run.sh`

*(SVARBU: Įsitikinkite, kad `run.sh` failas turi vykdymo (executable) teises. Daugeliu atvejų Home Assistant tai sutvarkys pats atsižvelgdamas į failo vidų, bet jei susidursite su problemomis - patikrinkite per SSH).*

### 4 žingsnis: Įskiepio įdiegimas
1. Home Assistant vartotojo sąsajoje eikite į **Settings** (Nustatymai) -> **Add-ons**.
2. Apatiniame dešiniajame kampe paspauskite **ADD-ON STORE** (Įskiepių parduotuvė).
3. Viršutiniame dešiniajame kampe paspauskite ant **Trijų taškelių piktogramos** ir pasirinkite **Check for updates** (Patikrinti atnaujinimus). Tai privers Home Assistant perskaityti `addons` katalogą.
4. Slinkite puslapį į patį viršų arba apačią, kur turėtumėte pamatyti naują skiltį pavadinimu **Local add-ons** (Vietiniai įskiepiai).
5. Paspauskite ant atsiradusio **IPTV Server** įskiepio.
6. Spauskite **INSTALL** (Įdiegti). Pirmas diegimas gali užtrukti kelias minutes, nes Home Assistant sistemoje bus parsiunčiamas Go kompiliatorius, kompiliuojamas jūsų kodas ir kuriamas Docker konteineris.

### 5 žingsnis: Nustatymai ir paleidimas
1. Įdiegus, **negalite** dar spausti Start. Pirmiausia nueikite į įskiepio viršutinį meniu ir paspauskite ant **Configuration** (Konfigūracija).
2. Čia matysite **Playlists** parametrą: kiekvienam grojaraščiui galite nurodyti **name** (pavadinimą) ir **url**. Pavadinimas naudojamas adrese: `/playlist/[pavadinimas].m3u`. Numatyta: `lit`, `rus` (iptv-org kanalai).
3. Galite pridėti, pakeisti ar ištrinti grojaraščius; pavadinimas turi būti URL-saugus (raidės, skaičiai, brūkšnys).
4. Išsaugokite nustatymus (**Save**) ir grįžkite į **Info** skirtuką.
5. Rekomenduojama pažymėti varneles prie:
   - **Start on boot** (savaime pasileis po Home Assistant perkrovimo).
   - **Watchdog** (savaime pasileis, jei įskiepis netikėtai sustos).
   - **Show in sidebar** (nebūtina, nes jis neturi Web UI, tik gražina .m3u failą).
6. Spauskite **START** (Paleisti).

### 6 žingsnis: Patikrinimas
1. Eikite į skirtuką **Log** (Žurnalas).
2. Turėtumėte pamatyti pranešimus apie paleistą srautą ir užklausas.
3. Norėdami naudoti IPTV grojaraštį savo televizoriuje (Apple TV ar kitoje programoje), nurodykite vieną iš adresų:
   - **Bendras grojaraštis** (visi šaltiniai sujungti):  
     `http://<JŪSŲ_HOME_ASSISTANT_IP>:8080/playlist.m3u`
   - **Atskiri grojaraščiai** (pagal konfigūracijoje nurodytą pavadinimą):  
     `http://<JŪSŲ_HOME_ASSISTANT_IP>:8080/playlist/[pavadinimas].m3u`  
     Pvz. su numatytais `lit` ir `rus`: `.../playlist/lit.m3u`, `.../playlist/rus.m3u`. Taip pat veikia pagal indeksą: `.../playlist/0.m3u`, `.../playlist/1.m3u`.
   *(Pvz.: `http://192.168.1.100:8080/playlist.m3u` arba `http://192.168.1.100:8080/playlist/lit.m3u`)*
