# Debian-Paketinstaller

`.deb`-Dateien werden im MiracleOS-Dateimanager direkt mit `miracleos-desktop --install DATEI` geöffnet. Der Starter `miracleos-installer.desktop` registriert den Installer außerdem für `application/vnd.debian.binary-package` und `application/x-deb`, damit Downloads auch aus anderen Apps geöffnet werden können. Er ist über das Startmenü und Einstellungen → Apps erreichbar.

Die Oberfläche liest Paketname, Version, Architektur und Beschreibung ohne Ausführung von Paketskripten. Vor der Installation ermittelt `apt-get --simulate --no-remove` die benötigten Abhängigkeiten. Fehlende oder beschädigte Dateien und nicht auflösbare Abhängigkeiten deaktivieren die Installation. Paketinformationen und Vorschau werden im Hintergrund geladen.

„Installieren“ startet APT über `pkexec` mit grafischer Passwortfreigabe durch den vorhandenen Polkit-Agenten. APT installiert die lokale Datei und Abhängigkeiten; `--no-remove` verhindert implizite Paketentfernungen. Bestehende Konfigurationsdateien bleiben erhalten. Fortschritt und Fehler erscheinen im Fenster. Der Installer prüft danach den dpkg-Status und bietet „App öffnen“ für Pakete mit Desktop-Starter an.

Voraussetzungen sind `pkexec`, `apt`, `dpkg`, ein laufender Polkit-Agent und `desktop-file-utils` für den Starter-Cache. Der ISO-Build enthält die zusätzlichen Pakete. Bereits installierte Systeme benötigen gegebenenfalls das Debian-Paket `pkexec`; das normale MiracleOS-Komponentenupdate installiert keine zusätzlichen Debian-Pakete.

Tests: `xvfb-run -a go test ./...`. Die neuen Tests bauen ein kleines lokales Paket mit `dpkg-deb` und prüfen Metadaten, Dateinamen mit Leerzeichen, file-URIs, beschädigte Dateien, Verzeichnisse und entfernte URI-Hosts. Die Paketprüfung darf dessen Installationsskript nicht ausführen. Die APT-Vorschau für `/home/florian/Downloads/Discord.deb` in der Test-VM kann Discord 1.0.166 amd64 und die benötigten Abhängigkeiten auflösen.
