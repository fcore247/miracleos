# HTML-Einstellungen

`--settings` öffnet eine eingebettete WebKitGTK-4.1-Oberfläche. HTML, CSS und JavaScript liegen in `cmd/desktop/settings-web/index.html` und werden mit `go:embed` in die Desktop-Binärdatei aufgenommen. Es sind kein Webserver, Node.js oder Internetzugriff zur Laufzeit erforderlich.

Die Oberfläche bietet Übersicht, Suche und alle bisherigen Kategorien. Darstellung, Schriftgröße, Transparenz, Hintergrund, globale und individuelle App-Fensterfarben verwenden weiterhin `systemPreferences` und dieselbe `settings.json`. Die bestehenden nativen Passwort-, Reset-, Update- und Dateiauswahldialoge werden weiterverwendet. Systeminformationen kommen weiterhin aus den vorhandenen Systemprogrammen. Die bisherige GTK-Oberfläche bleibt als interne Referenz `showNativeSettings` erhalten.

Die lokale Oberfläche sendet nacheinander JSON-Kommandos über Änderungen des WebView-Titels. Go prüft erlaubte Aktionen, Feldnamen und Werte; Rückgaben werden mit `encoding/json` serialisiert. Das Dokument hat eine restriktive Content Security Policy; Navigation zu anderen Seiten und neue Fenster sind blockiert. Passwörter werden ausschließlich in den bestehenden nativen Dialogen eingegeben.

## Bauen und prüfen

Zusätzlich zu den bestehenden Entwicklungsabhängigkeiten wird `libwebkit2gtk-4.1-dev` benötigt. Das ISO-Buildskript installiert dieses Paket; das Laufzeitpaket war bereits enthalten.

```sh
xvfb-run -a go test ./...
go build -o miracleos-desktop ./cmd/desktop
./miracleos-desktop --settings
./miracleos-desktop --settings Hintergrund
```

Prüfstand am 23.10.2026: JavaScript-Syntax und DOM-Funktionstests mit jsdom bestanden (Navigation durch alle Kategorien, Suche und Leerzustand, sichere Textausgabe, Hell/Dunkel, Akzent, Transparenz, App-Fensterfarbe, Anheften/Deinstallationsaufruf, Hintergrundaktionen, vier Updateaktionen sowie Passwort-/Resetaufrufe). Go-Quelldatei mit gofmt erfolgreich geparst und formatiert.

Anschließend direkt in der laufenden Debian-VM `MiracleOS-ISO-Test` mit Go 1.24.4 und WebKitGTK 2.54 gebaut: `go build ./cmd/desktop` und `xvfb-run -a go test ./...` erfolgreich. Die neue Binärdatei ist installiert; die Oberfläche wurde im laufenden Wayland-Desktop des Kontos `florian` geöffnet und visuell geprüft. Der Vergleich der Einstellungsdatei vor und nach dem Start war identisch. Passwortänderung, Reset und Updateinstallation wurden nicht ausgeführt.

Sicherung in der VM: `/opt/miracleos/backups/settings-html-20261023-091457/` enthält die vorherige Binärdatei, die bisherige `settings.json` und den ursprünglichen Binärpfad in `target.txt`. Für den Build wurden lokale Kopien von `desktoplib` und `libxdg-go` ausschließlich im temporären VM-Buildverzeichnis per `go mod edit -replace` eingebunden. Die Desktop-Leiste wurde nach dem Austausch neu gestartet, damit ihre Starter die neue Binärdatei verwenden.
