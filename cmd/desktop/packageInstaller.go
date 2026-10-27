package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

func showPackageInstaller(path string) {
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	win.SetTitle("Paketinstaller — MiracleOS")
	setUtilityWindowIcon(win, "package-installer")
	win.SetDefaultSize(660, 590)
	win.Connect("destroy", gtk.MainQuit)
	busy, alive := false, true
	var status *gtk.Label
	canClose := func() bool {
		if busy {
			status.SetText("Die Installation läuft noch. Bitte auf den Abschluss warten.")
			return false
		}
		return true
	}
	win.Connect("delete-event", func() bool { return !canClose() })
	root, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	menuClass(root, "fm-shell")
	root.PackStart(glassWindowHeader(win, "Paketinstaller — MiracleOS", canClose), false, false, 0)
	content, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 16)
	menuClass(content, "installer-content")
	icon := settingIcon("miracleos-package-installer", 64)
	icon.SetHAlign(gtk.ALIGN_CENTER)
	content.PackStart(icon, false, false, 0)
	heading := menuLabel("Paket öffnen", "installer-title")
	heading.SetXAlign(0.5)
	content.PackStart(heading, false, false, 0)
	subtitle := menuLabel("Debian-Programme für deinen Desktop", "installer-subtitle")
	subtitle.SetXAlign(0.5)
	subtitle.SetLineWrap(true)
	subtitle.SetMaxWidthChars(60)
	content.PackStart(subtitle, false, false, 0)
	card, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	menuClass(card, "installer-card")
	metadata := menuLabel("Wähle eine heruntergeladene .deb-Datei.", "installer-metadata")
	card.PackStart(metadata, false, false, 0)
	description := menuLabel("", "installer-description")
	description.SetLineWrap(true)
	description.SetMaxWidthChars(66)
	card.PackStart(description, false, false, 0)
	content.PackStart(card, false, false, 0)
	plan, _ := gtk.ExpanderNew("Installationsdetails und Abhängigkeiten")
	planText := settingsText("Noch kein Paket ausgewählt.")
	planScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	planScroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	planScroll.SetMinContentHeight(140)
	planScroll.Add(planText)
	plan.Add(planScroll)
	content.PackStart(plan, false, false, 0)
	status = menuLabel("Bereit", "installer-status")
	status.SetLineWrap(true)
	status.SetMaxWidthChars(65)
	content.PackStart(status, false, false, 0)
	progress, _ := gtk.ProgressBarNew()
	menuClass(progress, "installer-progress")
	content.PackStart(progress, false, false, 0)
	buttons, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	open, _ := gtk.ButtonNewWithLabel("Paket auswählen …")
	install, _ := gtk.ButtonNewWithLabel("Installieren")
	menuClass(open, "installer-secondary")
	menuClass(install, "installer-primary")
	install.SetSensitive(false)
	launch, _ := gtk.ButtonNewWithLabel("App öffnen")
	menuClass(launch, "installer-secondary")
	launch.SetNoShowAll(true)
	buttons.PackStart(launch, false, false, 0)
	buttons.PackStart(open, false, false, 0)
	buttons.PackEnd(install, false, false, 0)
	buttons.SetMarginStart(30)
	buttons.SetMarginEnd(30)
	buttons.SetMarginTop(12)
	buttons.SetMarginBottom(20)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scroll.Add(content)
	root.PackStart(scroll, true, true, 0)
	root.PackEnd(buttons, false, false, 0)
	win.Add(root)
	selected := ""
	packageName := ""
	win.Connect("destroy", func() { alive = false })
	load := func(file string) {
		selected, packageName = "", ""
		install.SetSensitive(false)
		open.SetSensitive(false)
		launch.Hide()
		heading.SetText("Paket wird geprüft …")
		description.SetText("")
		metadata.SetText("")
		planText.SetText("Abhängigkeiten werden geprüft …")
		status.SetText("Paketinformationen werden geladen. Es wird noch nichts installiert.")
		go func() {
			pkg, err := inspectDebPackage(file)
			preview := ""
			var previewErr error
			if err == nil {
				preview, previewErr = previewDebInstall(pkg.Path)
			}
			glib.IdleAdd(func() {
				if !alive {
					return
				}
				open.SetSensitive(true)
				if err != nil {
					heading.SetText("Paket konnte nicht geöffnet werden")
					status.SetText(err.Error())
					planText.SetText("Kein installierbares Paket ausgewählt.")
					return
				}
				selected, packageName = pkg.Path, pkg.Name
				heading.SetText(pkg.Name)
				metadata.SetText(fmt.Sprintf("Version %s    ·    %s    ·    %.1f MB", pkg.Version, pkg.Architecture, float64(pkg.Size)/(1024*1024)))
				lines := strings.Split(pkg.Description, "\n")
				if len(lines) > 5 {
					lines = lines[:5]
				}
				description.SetText(strings.Join(lines, "\n"))
				subtitle.SetText(filepath.Base(pkg.Path))
				planText.SetText(preview)
				progress.SetFraction(0)
				install.SetLabel("Installieren")
				message := "Bereit. Abhängigkeiten werden mitinstalliert; dein Passwort gibt die Installation frei."
				if pkg.InstalledVersion != "" {
					message = "Bereits installiert: " + pkg.InstalledVersion + ". Dieses Paket kann mit Passwortfreigabe erneut installiert oder aktualisiert werden."
					install.SetLabel("Aktualisieren / erneut installieren")
				}
				if previewErr != nil {
					status.SetText("Dieses Paket kann derzeit nicht installiert werden. Siehe Installationsdetails.")
					plan.SetExpanded(true)
					return
				}
				if _, e := exec.LookPath("pkexec"); e != nil {
					status.SetText("Die Passwortfreigabe fehlt. Bitte das Systempaket pkexec installieren.")
					return
				}
				status.SetText(message)
				install.SetSensitive(true)
			})
		}()
	}
	launch.Connect("clicked", func() {
		out, err := exec.Command("dpkg-query", "-L", packageName).Output()
		if err == nil {
			files := map[string]bool{}
			for _, file := range strings.Split(string(out), "\n") {
				files[file] = true
			}
			for _, app := range installedApplications() {
				if files[app.File] {
					launchInstalledApp(app)
					return
				}
			}
		}
		status.SetText("Dieses Paket hat keinen App-Starter. Installierte Apps findest du im Startmenü.")
	})
	open.Connect("clicked", func() {
		showFilePicker(&filePickerOptions{Title: "Debian-Paket auswählen", Parent: win, Filters: []pickerFilter{{Name: "Debian-Pakete (*.deb)", Rules: []pickerRule{{Kind: 0, Value: "*.deb"}}}}, Done: func(paths []string) {
			if len(paths) > 0 && alive {
				load(paths[0])
			}
		}})
	})
	install.Connect("clicked", func() {
		if selected == "" || busy {
			return
		}
		busy = true
		file, name := selected, packageName
		install.SetSensitive(false)
		open.SetSensitive(false)
		status.SetText("Bitte die Installation mit deinem Passwort freigeben …")
		glib.TimeoutAdd(200, func() bool {
			if !alive || !busy {
				return false
			}
			progress.Pulse()
			return true
		})
		go func() {
			command := exec.Command("pkexec", debInstallArguments(file)...)
			pipe, err := command.StdoutPipe()
			if err == nil {
				command.Stderr = command.Stdout
				err = command.Start()
			}
			last := ""
			if err == nil {
				scanner := bufio.NewScanner(pipe)
				scanner.Buffer(make([]byte, 65536), 1024*1024)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line == "" {
						continue
					}
					last = line
					message := line
					glib.IdleAdd(func() {
						if alive {
							status.SetText(message)
						}
					})
				}
				readErr := scanner.Err()
				if readErr != nil {
					_, _ = io.Copy(io.Discard, pipe)
				}
				err = command.Wait()
				if err == nil {
					err = readErr
				}
			}
			result := "Installation abgeschlossen."
			if err != nil {
				result = "Installation abgebrochen oder fehlgeschlagen."
				if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 126 {
					result = "Installation abgebrochen. Es wurde keine Freigabe erteilt."
				}
				if last != "" {
					result += "\n" + last
				}
			} else {
				installed, checkErr := exec.Command("dpkg-query", "-W", "-f=${Status}", name).Output()
				if checkErr != nil || !strings.Contains(string(installed), "install ok installed") {
					result = "Das Paket wurde noch nicht vollständig eingerichtet."
					err = fmt.Errorf("Paket nicht vollständig eingerichtet")
				}
			}
			glib.IdleAdd(func() {
				busy = false
				if !alive {
					return
				}
				status.SetText(result)
				progress.SetFraction(1)
				open.SetSensitive(true)
				install.SetSensitive(true)
				if err == nil {
					install.SetLabel("Erneut installieren")
					launch.Show()
				} else {
					install.SetLabel("Erneut versuchen")
					progress.SetFraction(0)
				}
			})
		}()
	})
	if path != "" {
		load(path)
	}
	win.ShowAll()
}
