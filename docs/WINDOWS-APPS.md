# Windows-Apps mit Wine

Der Dateimanager öffnet `.exe` und `.msi` mit `miracleos-desktop --windows DATEI`. Die MIME-Zuordnung `miracleos-windows.desktop` verbindet dieselbe Oberfläche mit „Öffnen“ bei Browser-Downloads, einschließlich Luna. Der Starter prüft lokale Dateien und unterstützt x86- und x86-64-Programme. Er verspricht keine Kompatibilität mit jedem Windows-Programm.

„Mit Wine starten“ bereitet beim ersten Start automatisch eine gemeinsame Windows-Umgebung unter `$XDG_DATA_HOME/miracleos/windows` (standardmäßig `~/.local/share/miracleos/windows`) vor. Parallele Starts werden während der Initialisierung per Dateisperre serialisiert. Die Windows-App läuft als normaler Benutzer; ausschließlich die einmalige Installation der Debian-Systempakete benötigt Administratorrechte. Ein Startprotokoll liegt unter `last-launch.log` im Windows-Verzeichnis. Wine ist keine Sandbox; Windows-Programme haben die Dateizugriffe des angemeldeten Benutzers.

## ISO

Der ISO-Build enthält Wine, Wine64 und die Wine-Schriften. `configure-image.sh` aktiviert die i386-Paketarchitektur und installiert zusätzlich `wine32:i386`. Dadurch sind 32- und 64-Bit-Programme ohne manuelle Nachinstallation nutzbar. Persönliche Wine-Umgebungen gehören nicht in das Image.

## Bestehende Installationen und signierte Updates

Die Erweiterung ist in der Desktop-Binärdatei enthalten und passt in das vorhandene signierte Komponentenpaket. Der Updater muss keine fremden Installationsskripte als root ausführen und benötigt kein neues Manifestformat. Die Wine-Systempakete werden auf älteren Installationen beim ersten Öffnen einer Windows-App automatisch nachinstalliert. Dazu zeigt die neue Oberfläche ein Passwortfeld; nach Bestätigung führt sie ausschließlich fest vorgegebene Debian-Paketverwaltungsaktionen aus. Es sind keine Terminalbefehle erforderlich. Eine Internetverbindung und ausreichend Speicher für die Wine-Pakete sind notwendig.

Das Release muss aus den aktuellen Quellen gebaut und mit einer höheren Sequenznummer signiert werden. Allein die lokale Testinstallation veröffentlicht kein GitHub-Release. Bestehende Windows-Umgebungen und Benutzerdateien bleiben bei Komponentenupdates erhalten.

## Deinstallation

„Deinstallieren“ in Einstellungen → Apps und im App-Kontextmenü erkennt Wine-Starter anhand ihrer Desktop-Datei und verwendet deren Wine-Umgebung. Der Starter wird als Argumentliste gelesen; seine Befehlszeile wird nicht in einer Shell ausgeführt. Ein eindeutiger Eintrag in `wine uninstaller --list` öffnet die passende Windows-Deinstallation über `--remove`. Bei uneindeutigen Namen öffnet sich stattdessen die Windows-Programmverwaltung zur Auswahl.

Ein Abbruch wird nicht als erfolgreiche Entfernung gemeldet. Erst wenn der Windows-Registrierungseintrag entfernt wurde, werden der ausgewählte Starter innerhalb der eigenen Anwendungsablage und dessen Anheftung entfernt. Andere Windows-Apps und die gemeinsame Wine-Umgebung bleiben bestehen. Die vorherigen transparenten App-Hinweisfenster wurden durch normale MiracleOS-Dialoge mit eigener Hintergrundfläche ersetzt.

Der Integrationstest `TestWineUninstallRuntime` erstellte am 26.10.2026 ein Testprogramm in einer separaten temporären Wine-Umgebung, fand dessen Registrierungseintrag, führte die Deinstallation aus und verifizierte die Entfernung erfolgreich. Die echten Programme des Benutzers wurden für diesen Test nicht deinstalliert. Zusätzliche Unit-Tests prüfen die Prefix-Erkennung, Namensmehrdeutigkeiten und das gezielte Aufräumen von Startern und Anheftungen.

## Tests

`xvfb-run -a go test ./...` prüft unter anderem gültige x86/x64-Dateien, beschädigte und fremde Architekturen, lokale Datei-URIs, Dateinamen mit Leerzeichen und Sonderzeichen, MSI-Argumente sowie parallele Erststarts.

Am 26.10.2026 wurden in der laufenden Debian-Test-VM Wine 10.0 inklusive `wine64` und `wine32:i386` installiert. Build und Go-Tests bestanden. Der zusätzliche Integrationstest `MIRACLEOS_WINE_TEST=1` führte die echten 32- und 64-Bit-Versionen von `cmd.exe` unter dem Benutzerkonto `florian` erfolgreich aus; die erste Windows-Umgebung wurde dabei automatisch erstellt. Die Dateizuordnungen für Windows-Programme zeigten auf `miracleos-windows.desktop`, und der Hinweisdialog wurde im laufenden Desktop visuell geprüft. Ein Start über den Dialog bereitete die persönliche Windows-Umgebung vor und öffnete `notepad.exe` sichtbar im Desktop. Luna verwendet für Downloads `xdg-open` und erhält dadurch denselben Starter.

Die finale Binärdatei wurde anschließend installiert und die Test-VM sauber heruntergefahren und wieder gestartet. Die veränderte Kernel-Boot-ID bestätigte den Neustart; Wine32, Wine64 und der Anmeldedienst waren danach verfügbar. Die VM stand wieder am Anmeldebildschirm. Die vorherige Desktop-Binärdatei liegt unter `/opt/miracleos/backups/windows-support-20261026-151943/`. Temporäre Verwaltungszugänge wurden danach geschlossen. Eine ISO und ein GitHub-Release wurden in diesem Arbeitsschritt noch nicht veröffentlicht.
