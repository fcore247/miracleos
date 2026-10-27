# MiracleOS

## Releases und Downloads

**Aktuelle Testversion: 27.10.2026 · amd64**

| Download | Link |
| --- | --- |
| Installations-ISO | [ISO herunterladen](https://github.com/fcore247/miracleos/releases/download/v0.1.0-test.20261027/MiracleOS-0.1-test.20261027-amd64.iso) |
| SHA-256-Prüfsumme | [Prüfsumme herunterladen](https://github.com/fcore247/miracleos/releases/download/v0.1.0-test.20261027/MiracleOS-0.1-test.20261027-SHA256SUMS.txt) |
| Änderungen und Teststand | [Release vom 27.10.2026](https://github.com/fcore247/miracleos/releases/tag/v0.1.0-test.20261027) |
| Frühere Versionen | [Alle Releases](https://github.com/fcore247/miracleos/releases) |

Das Repository und die Downloads sind öffentlich. Automatische Systemkomponenten-Updates werden separat als signierte Release-Pakete veröffentlicht; siehe [Updates einrichten](docs/UPDATES.md). Die neue ISO vom 27.10.2026 enthält die Update-Funktion bereits. Ältere Installationen können das Updater-Paket nachrüsten.

**Direkt zu:** [Installation](#installation) · [Funktionen](#funktionen) · [Entwicklung](#entwickeln-und-testen) · [Projektstruktur](#projektstruktur)

## Überblick

MiracleOS ist ein experimenteller Desktop für Debian 13 mit Go/GTK3, einem angepassten Sway-Compositor und einer installierbaren amd64-Live-ISO.

## Installation

Die ISO und ihre Prüfsumme findest du oben unter **Releases und Downloads**.

1. ISO herunterladen und die SHA-256-Prüfsumme mit der zugehörigen Prüfsummendatei vergleichen.
2. Zuerst in einer separaten VM starten (aktuelle Testkonfiguration: 4 CPUs, 8 GB RAM, 64 GB leere virtuelle Systemplatte).
3. Im Installer ausschließlich die dafür vorgesehene leere Platte auswählen. Die Installation löscht deren Inhalt.
4. Nach Installation die ISO auswerfen und von der Systemplatte starten.
5. Namen, Passwort und optional WLAN sowie Spotify einrichten.

BIOS und UEFI sind im Image vorgesehen. Secure Boot ist nicht eingerichtet. Der vollständige UEFI- und Reset-Test ist noch offen; siehe [Teststand](docs/TESTING.md).

## Neu in dieser Testversion

- Neue HTML-Einstellungen mit den bisherigen Einstellmöglichkeiten.
- Grafischer Debian-Paketinstaller mit Paketprüfung und Installationsvorschau.
- Windows-Apps per Doppelklick über Wine starten und über das Startmenü deinstallieren.
- Passende neue Icons für Installation und Deinstallation; Startmenü-Fokus beim Deinstallieren korrigiert.

## Funktionen

- Startmenü, Dock, Widgets und einheitliche Fensterränder mit abgesenkter Mitte.
- Eigener Dateimanager als Standard für Ordner sowie Öffnen-/Speichern-Dialoge über XDG Desktop Portal.
- Einstellungen für Hell/Dunkel und Akzentfarbe; der Anmeldebildschirm übernimmt die Kontoeinstellungen.
- Erste Einrichtung mit WLAN-Abfrage nur bei vorhandenem WLAN-Adapter.
- Offizielle Spotify-App, geführte Anmeldung und lokale Musiksteuerung.
- Spotify-Icon öffnet die echte App; **Alt + F** schaltet das Overlay ein/aus. Keine Spotify-Web-API-Suche und keine zusätzlichen API-Zugangsdaten.
- Reset-Ablauf mit Bestätigung, Passwortprüfung und zweiter Bestätigung; benötigt die passende ISO und löscht die Systemplatte.
- Automatische, signierte Updates aus GitHub Releases mit Statusanzeige, Pausieren und Wiederherstellung der vorherigen Version.

## Entwickeln und testen

Auf Linux mit Go 1.24 oder neuer:

```sh
sudo apt install build-essential pkg-config libgtk-3-dev libgtk-layer-shell-dev libvte-2.91-dev libglib2.0-dev xvfb dbus-x11
xvfb-run -a go test ./...
MIRACLEOS_DBUS_TEST=1 dbus-run-session -- xvfb-run -a go test ./cmd/desktop -run TestSpotifyDBus -count=1
python3 -m unittest discover -s scripts/iso -p 'test_*.py'
go build -o miracleos-desktop ./cmd/desktop
```

Zum ISO-Build eine wegwerfbare Debian-13-Builder-VM verwenden:

```sh
sudo bash scripts/iso/build.sh /home/builder/miracleos-live /path/to/tested/luna-browser
```

Das Skript installiert Build-Abhängigkeiten, baut den angepassten Compositor, Desktop und Tab-Dienst, integriert das getestete Luna-Linux-Binary, führt Tests aus und erzeugt `live-image-amd64.hybrid.iso` sowie `SHA256SUMS`. Der Build benötigt Internet und ausreichend freien Speicher. Debian- und Spotify-Paketquellen sind zeitabhängig; das Ergebnis ist nicht bitgenau reproduzierbar.

## Projektstruktur

| Pfad | Inhalt |
| --- | --- |
| `cmd/desktop/` | GTK-Oberfläche, Explorer, Portal, Spotify, Einrichtung, Tests und eingebettete Ressourcen |
| `compositor/` | Sway-Anpassungen und Buildskripte |
| `runtime/` | Hilfsprogramme für die installierte Sitzung |
| `scripts/iso/` | Live-Image, Installation, Reset und Sicherheitstests |
| `scripts/install/` | Historische Entwicklungsskripte; keine allgemeine Installationsanleitung |
| `docs/` | Architektur, Bedienung, Teststand und Release-Hinweise |

Weiterlesen: [ISO und Reset](docs/ISO.md), [Architektur](docs/ARCHITECTURE.md), [Teststand](docs/TESTING.md), [Release-Ablauf](docs/RELEASING.md).

## Sicherheit und Lizenz

Keine VM-Festplatten, privaten SSH-Schlüssel, Passwörter, Spotify-Profile oder persönliche Dateien gehören in dieses Repository oder die ISO. Fehlerberichte sollen keine Zugangsdaten enthalten.

Die vorhandene GPL-3.0-Lizenz und ursprünglichen Urheberhinweise bleiben erhalten. Der Go-Modulpfad verweist weiterhin auf das ursprüngliche Projekt `github.com/MiracleOS-Team/desktop`. Spotify und weitere enthaltene Pakete haben eigene Lizenzen; ein privates Repository ersetzt keine Weitergaberechte.
