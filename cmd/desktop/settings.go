package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type settingCategory struct{ name, icon string }

//go:embed images/settings-icons/*.png
var settingsIconFiles embed.FS
var settingCategories = []settingCategory{
	{"System", "computer"}, {"Geräte", "preferences-desktop-peripherals"}, {"Netzwerk", "network-workgroup"},
	{"Apps", "applications-other"}, {"Konten", "system-users"}, {"Personalisierung", "preferences-desktop-theme"},
	{"Zeit & Sprache", "preferences-desktop-locale"}, {"Barrierefreiheit", "preferences-desktop-accessibility"},
	{"Updates", "software-update-available"}, {"Datenschutz", "changes-prevent"},
}

type settingsUI struct {
	pageName string
	win      *gtk.Window
	body     *gtk.Box
	view     *gtk.Box
	status   *gtk.Label
	alive    bool
}

func launchSettings() {
	hideStartMenu()
	exe, err := os.Executable()
	if err == nil {
		_ = exec.Command(exe, "--settings").Start()
	}
}

func settingsButton(text string, action func()) *gtk.Button {
	b, _ := gtk.ButtonNewWithLabel(text)
	menuClass(b, "settings-button")
	b.Connect("clicked", action)
	return b
}
func settingsCard(title string) *gtk.Box {
	b, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	menuClass(b, "settings-card")
	b.PackStart(menuLabel(title, "settings-card-title"), false, false, 0)
	return b
}
func settingsText(text string) *gtk.Label {
	l := menuLabel(text, "settings-muted")
	l.SetLineWrap(true)
	l.SetMaxWidthChars(48)
	l.SetSelectable(true)
	return l
}
func settingIcon(name string, size int) *gtk.Image {
	if name == "miracleos-uninstall" || name == "miracleos-package-installer" {
		id := "uninstall"
		if name == "miracleos-package-installer" { id = "package-installer" }
		if data, err := settingsIconFiles.ReadFile("images/settings-icons/" + id + ".png"); err == nil {
			if icon, err := gdk.PixbufNewFromBytesOnly(data); err == nil {
				image, _ := gtk.ImageNewFromPixbuf(scalePixbuf(icon, size, size))
				return image
			}
		}
		name = "edit-delete"
	}
	ids := map[string]string{"preferences-system": "settings", "computer": "system", "preferences-desktop-peripherals": "devices", "network-workgroup": "network", "applications-other": "apps", "system-users": "accounts", "preferences-desktop-theme": "personalization", "preferences-desktop-locale": "time", "preferences-desktop-accessibility": "accessibility", "software-update-available": "updates", "changes-prevent": "privacy"}
	if id, ok := ids[name]; ok {
		if data, err := settingsIconFiles.ReadFile("images/settings-icons/" + id + ".png"); err == nil {
			if pixbuf, err := gdk.PixbufNewFromBytesOnly(data); err == nil {
				image, _ := gtk.ImageNewFromPixbuf(scalePixbuf(pixbuf, size, size))
				return image
			}
		}
	}
	if name == "preferences-system" {
		image, _ := gtk.ImageNewFromIconName("preferences-system-symbolic", gtk.ICON_SIZE_DIALOG)
		image.SetPixelSize(size)
		return image
	}
	image, _ := gtk.ImageNewFromIconName(name, gtk.ICON_SIZE_DIALOG)
	image.SetPixelSize(size)
	return image
}

func showNativeSettings() {
	ui := &settingsUI{alive: true}
	ui.win, _ = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	ui.win.SetTitle("Einstellungen — MiracleOS")
	ui.win.SetDefaultSize(1080, 650)
	root, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	menuClass(root, "fm-shell")
	menuClass(root, "settings-shell")
	root.PackStart(glassWindowHeader(ui.win, "Einstellungen — MiracleOS"), false, false, 0)
	ui.body, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	root.PackStart(ui.body, true, true, 0)
	ui.win.Add(root)
	ui.win.Connect("destroy", func() { ui.alive = false; gtk.MainQuit() })
	if len(os.Args) > 2 {
		ui.detail(os.Args[2])
	} else {
		ui.home()
	}
	ui.win.ShowAll()
	lastDark := readSystemPreferences().Dark
	glib.TimeoutAdd(600, func() bool {
		if !ui.alive {
			return false
		}
		dark := readSystemPreferences().Dark
		if dark != lastDark {
			lastDark = dark
			if ui.pageName == "Apps" {
				ui.detail("Apps")
			}
		}
		return true
	})
}

func (ui *settingsUI) begin() *gtk.Box {
	if ui.view != nil {
		ui.view.Destroy()
	}
	ui.view, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 16)
	menuClass(ui.view, "settings-page")
	ui.body.PackStart(ui.view, true, true, 0)
	return ui.view
}

func (ui *settingsUI) search() *gtk.SearchEntry {
	search, _ := gtk.SearchEntryNew()
	search.SetPlaceholderText("Einstellungen durchsuchen")
	search.SetWidthChars(30)
	menuClass(search, "settings-search")
	search.Connect("activate", func() {
		q, _ := search.GetText()
		q = strings.ToLower(strings.TrimSpace(q))
		if q == "" {
			return
		}
		if strings.Contains(q, "farb") || strings.Contains(q, "theme") || strings.Contains(q, "dunkel") || strings.Contains(q, "hell") {
			ui.detail("Personalisierung")
			return
		}
		if strings.Contains(q, "hintergrund") || strings.Contains(q, "wallpaper") {
			ui.detail("Hintergrund")
			return
		}
		for _, c := range settingCategories {
			if strings.Contains(strings.ToLower(c.name), q) {
				ui.detail(c.name)
				return
			}
		}
		ui.detail("Suche: " + q)
	})
	return search
}

func (ui *settingsUI) home() {
	ui.pageName = ""
	page := ui.begin()
	top, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 20)
	profile, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	profile.PackStart(settingIcon("system-users", 40), false, false, 0)
	userInfo, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 3)
	current, _ := user.Current()
	username := "test"
	if current != nil {
		username = current.Username
	}
	userInfo.PackStart(menuLabel(username, "settings-card-title"), false, false, 0)
	userInfo.PackStart(settingsText("Lokales Konto"), false, false, 0)
	profile.PackStart(userInfo, false, false, 0)
	top.PackStart(profile, true, true, 0)
	title := menuLabel("Einstellungen", "settings-title")
	title.SetHAlign(gtk.ALIGN_CENTER)
	top.PackStart(title, true, true, 0)
	quick, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	for _, item := range []struct{ title, icon, page string }{{"System", "computer", "System"}, {"Updates", "software-update-available", "Updates"}, {"Personalisierung", "preferences-desktop-theme", "Personalisierung"}} {
		item := item
		b, _ := gtk.ButtonNew()
		menuClass(b, "settings-category")
		box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
		box.PackStart(settingIcon(item.icon, 28), false, false, 0)
		box.PackStart(menuLabel(item.title, "settings-small"), false, false, 0)
		b.Add(box)
		b.Connect("clicked", func() { ui.detail(item.page) })
		quick.PackStart(b, false, false, 0)
	}
	top.PackEnd(quick, true, true, 0)
	page.PackStart(top, false, false, 2)
	search := ui.search()
	search.SetHAlign(gtk.ALIGN_CENTER)
	page.PackStart(search, false, false, 0)
	tags, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 7)
	tags.SetHAlign(gtk.ALIGN_CENTER)
	for _, name := range []string{"#Personalisierung", "#Systemfarben", "#Datenschutz"} {
		name := name
		b := settingsButton(name, func() {
			target := strings.TrimPrefix(name, "#")
			if target == "Systemfarben" {
				target = "Personalisierung"
			}
			ui.detail(target)
		})
		menuClass(b, "settings-chip")
		tags.PackStart(b, false, false, 0)
	}
	page.PackStart(tags, false, false, 0)
	grid, _ := gtk.GridNew()
	grid.SetColumnHomogeneous(true)
	grid.SetColumnSpacing(12)
	grid.SetHExpand(true)
	network := settingsCard("◆  Netzwerk")
	network.PackStart(settingsText("Virtueller Adapter"), false, false, 0)
	netLabel := settingsText("")
	network.PackStart(netLabel, true, true, 0)
	network.PackEnd(settingsButton("Netzwerk öffnen  ›", func() { ui.detail("Netzwerk") }), false, false, 0)
	devices := settingsCard("◈  Bluetooth")
	devices.PackStart(settingsText("Geräte in dieser VM"), false, false, 0)
	btLabel := settingsText("")
	devices.PackStart(btLabel, true, true, 0)
	devices.PackEnd(settingsButton("Geräte öffnen  ›", func() { ui.detail("Geräte") }), false, false, 0)
	personalization, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	wallpaper := settingsCard("▧  Hintergrund")
	thumbs, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	home, _ := os.UserHomeDir()
	wallpaperPath := readSystemPreferences().Wallpaper
	if wallpaperPath == "" {
		wallpaperPath = filepath.Join(home, ".local/share/backgrounds/miracleos-wallpaper.png")
	}
	if pixbuf, err := gdk.PixbufNewFromFile(wallpaperPath); err == nil {
		image, _ := gtk.ImageNewFromPixbuf(scalePixbuf(pixbuf, 164, 68))
		thumbs.PackStart(image, false, false, 0)
	}
	wallpaper.PackStart(thumbs, false, false, 0)
	wallpaper.PackStart(settingsButton("Hintergrund wählen  ›", func() { ui.detail("Hintergrund") }), false, false, 0)
	personalization.PackStart(wallpaper, true, true, 0)
	personalization.PackStart(settingsButton("◈  Theme & Farben  ›", func() { ui.detail("Personalisierung") }), false, false, 0)
	software := settingsCard("◇  Software")
	software.PackStart(settingsButton("Apps  ›", func() { ui.detail("Apps") }), false, false, 0)
	software.PackStart(settingsText("Debian-Pakete und MiracleOS-Apps"), true, true, 0)
	software.PackStart(settingsButton("Updates prüfen  ›", func() { ui.detail("Updates") }), false, false, 0)
	grid.Attach(network, 0, 0, 1, 1)
	grid.Attach(devices, 1, 0, 1, 1)
	grid.Attach(personalization, 2, 0, 1, 1)
	grid.Attach(software, 3, 0, 1, 1)
	page.PackStart(grid, true, true, 10)
	categories, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	menuClass(categories, "settings-category-row")
	for _, c := range settingCategories {
		c := c
		b, _ := gtk.ButtonNew()
		menuClass(b, "settings-category")
		box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
		box.PackStart(settingIcon(c.icon, 32), false, false, 0)
		label := menuLabel(c.name, "settings-small")
		label.SetXAlign(.5)
		label.SetLineWrap(true)
		label.SetMaxWidthChars(13)
		box.PackStart(label, false, false, 0)
		b.Add(box)
		b.Connect("clicked", func() { ui.detail(c.name) })
		categories.PackStart(b, true, true, 0)
	}
	page.PackEnd(categories, false, false, 0)
	page.ShowAll()
	go func() {
		net := "Kein Netzwerkadapter verfügbar."
		addresses := settingsCommand("ip", "-4", "-o", "addr", "show", "scope", "global")
		for _, line := range strings.Split(addresses, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				net = "Ethernet · " + fields[1] + "\n" + fields[3] + "\nVirtualBox-Verbindung"
				break
			}
		}
		bt := settingsCommand("bluetoothctl", "list")
		glib.IdleAdd(func() {
			if !ui.alive || ui.view != page {
				return
			}
			netLabel.SetText(net)
			if bt == "" || strings.Contains(bt, "nicht verfügbar") {
				btLabel.SetText("Kein Bluetooth-Adapter in dieser VM.")
			} else {
				btLabel.SetText(bt)
			}
		})
	}()
}

func (ui *settingsUI) detail(name string) {
	ui.pageName = name
	if name == "Farben" {
		name = "Personalisierung"
	}
	page := ui.begin()
	toolbar, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 14)
	toolbar.PackStart(settingsButton("‹  Übersicht", ui.home), false, false, 0)
	toolbar.PackStart(menuLabel("Einstellungen  ›  "+name, "settings-card-title"), true, true, 0)
	toolbar.PackEnd(ui.search(), false, false, 0)
	page.PackStart(toolbar, false, false, 0)
	main, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 18)
	sidebar, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 3)
	menuClass(sidebar, "settings-sidebar")
	for _, c := range settingCategories {
		c := c
		b := settingsButton(c.name+"  ›", func() { ui.detail(c.name) })
		if c.name == name {
			menuClass(b, "settings-selected")
		}
		sidebar.PackStart(b, false, false, 0)
		if c.name == "Personalisierung" {
			for _, sub := range []string{"Hintergrund"} {
				sub := sub
				b := settingsButton("    "+sub, func() { ui.detail(sub) })
				if sub == name {
					menuClass(b, "settings-selected")
				}
				sidebar.PackStart(b, false, false, 0)
			}
		}
	}
	main.PackStart(sidebar, false, false, 0)
	content, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 14)
	content.SetHExpand(true)
	content.PackStart(menuLabel(name, "settings-title"), false, false, 0)
	ui.status = menuLabel("", "settings-muted")
	switch name {
	case "Personalisierung", "Barrierefreiheit":
		ui.colors(content)
	case "Hintergrund":
		ui.wallpaper(content)
	default:
		ui.information(content, name)
	}
	content.PackEnd(ui.status, false, false, 0)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scroll.Add(content)
	main.PackStart(scroll, true, true, 0)
	page.PackStart(main, true, true, 0)
	page.ShowAll()
}

func (ui *settingsUI) save(p systemPreferences) {
	p = modeWallpaper(p)
	previous := readSystemPreferences().Wallpaper
	if err := saveSystemPreferences(p); err != nil {
		ui.status.SetText("Speichern fehlgeschlagen: " + err.Error())
	} else {
		if previous != p.Wallpaper {
			if err := applyWallpaper(p.Wallpaper); err != nil {
				ui.status.SetText("Hintergrund konnte nicht übernommen werden.")
				return
			}
		}
		ui.status.SetText("Gespeichert · wird systemweit übernommen")
	}
}

func (ui *settingsUI) colors(content *gtk.Box) {
	p := readSystemPreferences()
	columns, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 14)
	left, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 14)
	left.SetHExpand(true)
	appearance := settingsCard("Darstellung")
	previews, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	for _, dark := range []bool{false, true} {
		dark := dark
		title := "Helles Theme"
		class := "settings-theme-sample-light"
		if dark {
			title = "Dunkles Theme"
			class = "settings-theme-sample-dark"
		}
		b, _ := gtk.ButtonNew()
		menuClass(b, "settings-theme-preview")
		box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
		sample, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
		menuClass(sample, class)
		sample.SetSizeRequest(150, 85)
		sample.PackStart(menuLabel("Aa", "settings-preview-type"), false, false, 4)
		sample.PackStart(menuLabel("Dein Desktop", "settings-small"), false, false, 0)
		box.PackStart(sample, false, false, 0)
		caption := menuLabel(title, "settings-small")
		caption.SetXAlign(.5)
		box.PackStart(caption, false, false, 0)
		b.Add(box)
		b.SetTooltipText(title + " aktivieren")
		b.Connect("clicked", func() { current := readSystemPreferences(); current.Dark = dark; ui.save(current) })
		previews.PackStart(b, true, true, 0)
	}
	appearance.PackStart(previews, false, false, 0)
	left.PackStart(appearance, false, false, 0)
	behavior := settingsCard("Glas & Lesbarkeit")
	row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	row.PackStart(menuLabel("Transparenzeffekt", ""), true, true, 0)
	toggle, _ := gtk.SwitchNew()
	toggle.SetActive(p.Transparency)
	toggle.SetVAlign(gtk.ALIGN_CENTER)
	toggle.Connect("notify::active", func() {
		current := readSystemPreferences()
		current.Transparency = toggle.GetActive()
		ui.save(current)
	})
	row.PackEnd(toggle, false, false, 0)
	behavior.PackStart(row, false, false, 0)
	fontRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	fontRow.PackStart(menuLabel("Schriftgröße", ""), true, true, 0)
	font, _ := gtk.ComboBoxTextNew()
	for _, label := range []string{"Klein", "Mittel", "Groß"} {
		font.AppendText(label)
	}
	active := 1
	if p.FontSize <= 10 {
		active = 0
	}
	if p.FontSize >= 14 {
		active = 2
	}
	font.SetActive(active)
	font.Connect("changed", func() {
		current := readSystemPreferences()
		sizes := []int{10, 12, 14}
		i := font.GetActive()
		if i >= 0 && i < len(sizes) {
			current.FontSize = sizes[i]
			ui.save(current)
		}
	})
	fontRow.PackEnd(font, false, false, 0)
	behavior.PackStart(fontRow, false, false, 0)
	behavior.PackStart(settingsText("Die Auswahl bleibt nach einem Neustart erhalten."), false, false, 0)
	left.PackStart(behavior, false, false, 0)
	reset := settingsButton("Standard wiederherstellen", func() {
		current := readSystemPreferences()
		current.Accent = "#bf69a4"
		current.Dark = false
		current.Transparency = true
		current.FontSize = 12
		ui.save(current)
		ui.detail("Personalisierung")
	})
	left.PackStart(reset, false, false, 0)
	colors := settingsCard("Systemfarbe")
	colors.SetHExpand(true)
	colors.PackStart(settingsText("Wähle deine Akzentfarbe"), false, false, 0)
	swatches, _ := gtk.GridNew()
	swatches.SetColumnSpacing(10)
	swatches.SetRowSpacing(12)
	palette := []struct{ hex, name string }{{"#bf69a4", "Rosa"}, {"#ec4779", "Pink"}, {"#3d9894", "Türkis"}, {"#8183e7", "Lavendel"}, {"#ad62d7", "Violett"}, {"#e8ae29", "Gold"}, {"#ee9235", "Orange"}, {"#d35c45", "Koralle"}, {"#dc3a53", "Rot"}, {"#ab3e79", "Beere"}, {"#2389cc", "Blau"}, {"#226fa8", "Ozean"}, {"#36a7ba", "Cyan"}, {"#2d9b79", "Mint"}, {"#378957", "Grün"}}
	buttons := []*gtk.Button{}
	for i, color := range palette {
		color := color
		b, _ := gtk.ButtonNewWithLabel(" ")
		menuClass(b, "settings-swatch")
		b.SetTooltipText(color.name)
		provider, _ := gtk.CssProviderNew()
		_ = provider.LoadFromData(fmt.Sprintf("button { background-image:none; background-color:%s; } button:hover { background-color:%s; }", color.hex, color.hex))
		ctx, _ := b.GetStyleContext()
		ctx.AddProvider(provider, gtk.STYLE_PROVIDER_PRIORITY_USER)
		if color.hex == p.Accent {
			b.SetLabel("✓")
		}
		b.Connect("clicked", func() {
			current := readSystemPreferences()
			current.Accent = color.hex
			ui.save(current)
			for _, other := range buttons {
				other.SetLabel(" ")
			}
			b.SetLabel("✓")
		})
		buttons = append(buttons, b)
		swatches.Attach(b, i%5, i/5, 1, 1)
	}
	colors.PackStart(swatches, false, false, 10)
	colors.PackEnd(settingsText("Dateien, Installer, Startmenü und neue Passwortabfragen verwenden dieselbe Farbpalette."), false, false, 0)
	columns.PackStart(left, true, true, 0)
	columns.PackStart(colors, true, true, 0)
	content.PackStart(columns, false, false, 0)
}

func (ui *settingsUI) wallpaper(content *gtk.Box) {
	card := settingsCard("Dein Hintergrund")
	p := readSystemPreferences()
	path := p.Wallpaper
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".local/share/backgrounds/miracleos-wallpaper.png")
	}
	if pixbuf, err := gdk.PixbufNewFromFile(path); err == nil {
		image, _ := gtk.ImageNewFromPixbuf(scalePixbuf(pixbuf, 530, 270))
		card.PackStart(image, false, false, 0)
	}
	card.PackStart(settingsText(filepath.Base(path)), false, false, 0)
	choose := settingsButton("Bild auswählen …", func() {
		showFilePicker(&filePickerOptions{Title: "Hintergrund auswählen", Parent: ui.win, Filters: []pickerFilter{{Name: "Bilder (PNG, JPEG, WebP, GIF)", Rules: []pickerRule{{Kind: 0, Value: "*.png"}, {Kind: 0, Value: "*.jpg"}, {Kind: 0, Value: "*.jpeg"}, {Kind: 0, Value: "*.webp"}, {Kind: 0, Value: "*.gif"}}}}, Done: func(paths []string) {
			if len(paths) == 0 || !ui.alive {
				return
			}
			selected := paths[0]
			if _, err := gdk.PixbufNewFromFile(selected); err != nil {
				ui.status.SetText("Dieses Bild kann nicht geöffnet werden.")
				return
			}
			current := readSystemPreferences()
			current.Wallpaper = selected
			current.CustomWallpaper = true
			ui.save(current)
			if err := applyWallpaper(selected); err != nil {
				ui.status.SetText("Hintergrund konnte nicht angewendet werden.")
			} else {
				ui.detail("Hintergrund")
			}
		}})
	})
	menuClass(choose, "settings-primary")
	card.PackStart(choose, false, false, 0)
	card.PackStart(settingsButton("Standard für Hell / Dunkel verwenden", func() {
		current := readSystemPreferences()
		current.CustomWallpaper = false
		current.Wallpaper = ""
		ui.save(current)
		ui.detail("Hintergrund")
	}), false, false, 0)
	content.PackStart(card, false, false, 0)
}

func applyWallpaper(path string) error {
	// Quoted Sway arguments support spaces without invoking a shell.
	quoted := fmt.Sprintf("%q", path)
	if err := exec.Command("swaymsg", "output * bg "+quoted+" fill").Run(); err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	config := filepath.Join(home, ".config/sway/config")
	data, err := os.ReadFile(config)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "output * bg ") {
			lines[i] = "output * bg " + quoted + " fill"
			found = true
		}
	}
	if !found {
		lines = append(lines, "output * bg "+quoted+" fill")
	}
	return os.WriteFile(config, []byte(strings.Join(lines, "\n")), 0644)
}

func settingsCommand(command string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, command, args...).Output()
	if err != nil {
		return "In dieser VM nicht verfügbar."
	}
	return strings.TrimSpace(string(data))
}

func (ui *settingsUI) information(content *gtk.Box, name string) {
	if name == "Updates" {
		ui.updates(content)
		return
	}
	if name == "Apps" {
		ui.frameColors(content)
		search, _ := gtk.EntryNew()
		search.SetPlaceholderText("Installierte Apps durchsuchen …")
		menuClass(search, "fm-search")
		content.PackStart(search, false, false, 0)
		list := installedAppsView(ui.win, search, ui)
		list.SetMinContentHeight(300)
		content.PackStart(list, true, true, 0)
		content.PackStart(settingsButton("Paketinstaller öffnen", func() { exe, _ := os.Executable(); _ = exec.Command(exe, "--install").Start() }), false, false, 0)
		return
	}
	if name == "System" {
		accountCard := settingsCard("Anmeldung")
		accountCard.PackStart(settingsButton("Passwort ändern …", ui.passwordDialog), false, false, 0)
		accountCard.PackStart(settingsButton("MiracleOS vollständig zurücksetzen …", ui.resetSystemDialog), false, false, 0)
		content.PackStart(accountCard, false, false, 0)
	}
	view := ui.view
	card := settingsCard(name)
	text := settingsText("Informationen werden geladen …")
	card.PackStart(text, false, false, 0)
	content.PackStart(card, false, false, 0)
	if name == "Apps" {
		card.PackStart(settingsButton("Paketinstaller öffnen", func() { _ = exec.Command("/home/test/desktop/miracleos-desktop", "--install").Start() }), false, false, 0)
	}
	load := func() {
		go func() {
			result := ""
			switch name {
			case "System":
				host, _ := os.Hostname()
				result = "MiracleOS Desktop\nDebian GNU/Linux\n\nGerät: " + host + "\nKernel: " + settingsCommand("uname", "-r") + "\nArchitektur: " + settingsCommand("uname", "-m")
			case "Netzwerk":
				result = settingsCommand("nmcli", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device") + "\n\nDie VM verwendet den virtuellen Netzwerkadapter. WLAN-Netze des Windows-PCs werden hier nicht direkt angezeigt."
			case "Geräte":
				result = "USB-Geräte\n" + settingsCommand("lsusb") + "\n\nBluetooth\n" + settingsCommand("bluetoothctl", "list")
			case "Apps":
				result = "Installierte Anwendungen\n\n" + settingsCommand("sh", "-c", "find /usr/share/applications /usr/local/share/applications -maxdepth 1 -name '*.desktop' -printf '%f\\n' | sort")
			case "Konten":
				current, _ := user.Current()
				if current != nil {
					result = "Benutzer: " + current.Username + "\nPersönlicher Ordner: " + current.HomeDir + "\n\nLokales Debian-Konto"
				}
			case "Zeit & Sprache":
				result = time.Now().Format("02.01.2006 · 15:04") + "\n\n" + settingsCommand("timedatectl", "status")
			case "Updates":
				result = settingsCommand("apt", "list", "--upgradable") + "\n\nDiese Liste verwendet den vorhandenen Debian-Paketcache. Es werden keine Updates automatisch installiert."
			case "Datenschutz":
				result = "Lokale Einstellungen\n\nSystemfarben und Darstellungsoptionen werden in deinem Benutzerordner gespeichert. Diese Einstellungs-App sendet keine Nutzungsdaten.\n\nMiracleOS ist hier ein Desktop auf Debian; installierte Programme haben eigene Datenschutzoptionen."
			default:
				result = "Keine passende Einstellung gefunden. Versuche beispielsweise Farben, Hintergrund, Apps oder Netzwerk."
			}
			glib.IdleAdd(func() {
				if ui.alive && ui.view == view {
					text.SetText(result)
				}
			})
		}()
	}
	if name == "Updates" || name == "Netzwerk" || name == "Geräte" {
		card.PackStart(settingsButton("Erneut prüfen", load), false, false, 0)
	}
	load()
}
