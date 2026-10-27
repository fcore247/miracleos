# Teststand

## Testversion vom 27. Oktober 2026

Der aktuelle Desktop mit HTML-Einstellungen, Debian-Paketinstaller und Wine-Integration bestand die Go-Tests auf dem separaten Debian-13-Builder. Die 14 Python-Tests für Installation und Updates sowie der isolierte Spotify-D-Bus-Test bestanden ebenfalls. Der Compositor wurde aus frischen Sway-Quellen neu gebaut.

Die Tests vom 26. Oktober zur Wine-Installation und -Deinstallation sind in [Windows-Apps](WINDOWS-APPS.md) dokumentiert. Einzelheiten zum Boot-Test des veröffentlichten ISO-Abbilds stehen zusätzlich in dessen Release-Beschreibung. Ein Boot-Test ersetzt keinen vollständigen Installations- oder Reset-Test.

Stand: 9. Oktober 2026. Experimenteller Teststand, keine allgemeine Produktionsfreigabe.

## Bereits geprüft

- Go-Tests, Python-Tests für Installer/Profilübernahme und gesonderte Spotify-D-Bus-Tests in bisherigen Builds.
- BIOS-Installation in separater VirtualBox-VM.
- Echte Spotify-Anmeldung sichtbar, QR-Hinweis und Rahmenlayout.
- MPRIS-Wiedergabestatus einschließlich Wiedergabe auf einem anderen Spotify-Connect-Gerät.
- Neue runde Overlay-Buttons und schlanke Slider in der Test-VM sichtbar geprüft.
- Eigener Speichern-Dialog über direkten D-Bus-Aufruf sichtbar.
- Eigener Dateimanager per inode/directory als Standard bestätigt; Launcher und fehlendes gio ergänzt.

## Vor einer breiten Freigabe prüfen

- Frische Installation der exakt veröffentlichten ISO, Einrichtung vollständig durchlaufen und erneut anmelden.
- Theme-Übernahme im Greeter in beiden Modi und mit unterschiedlichen Akzenten.
- Firefox neu starten, Datei speichern/öffnen, Abbrechen und Überschreibbestätigung vollständig prüfen. Ein direkter Abbruch-Test über D-Bus hatte einen Timeout; Ursache noch offen.
- Vollbild während Spotify-Einrichtung und automatischer Übergang zum Widget nach Wiedergabestart erneut prüfen.
- Echte WLAN-Verbindung auf Hardware; VM besitzt nur virtuelle Ethernet-Karte.
- UEFI-Installation und kompletter destruktiver Reset-Zyklus in wegwerfbarer VM mit zusätzlicher Sentinel-Platte.
- Neu angelegte Benutzer: Standard-Dock, Dateien-/Einstellungen-Icons, Standard-Dateimanager und Portal-Aktivierung.

Details eines Builds gehören zusätzlich in die Release-Beschreibung und Prüfsummendatei. Ein erfolgreicher Build allein bestätigt keine erfolgreiche Installation.
