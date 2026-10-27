package main

/*
#cgo pkg-config: webkit2gtk-4.1
#include <webkit2/webkit2.h>
#include <stdint.h>
#include <stdlib.h>
extern void settingsWebCommand(uintptr_t handle, char *message);
static void settings_title(WebKitWebView *view, GParamSpec *spec, gpointer data) {
 const char *title = webkit_web_view_get_title(view);
 if (title) settingsWebCommand((uintptr_t)data, (char*)title);
}
static gboolean settings_policy(WebKitWebView *view, WebKitPolicyDecision *decision, WebKitPolicyDecisionType type, gpointer data) {
 if (type == WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION) { webkit_policy_decision_ignore(decision); return TRUE; }
 if (type == WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION) {
  WebKitURIRequest *request = webkit_navigation_policy_decision_get_request(WEBKIT_NAVIGATION_POLICY_DECISION(decision));
  if (g_strcmp0(webkit_uri_request_get_uri(request), "about:blank") != 0) { webkit_policy_decision_ignore(decision); return TRUE; }
 }
 return FALSE;
}
static WebKitWebView *settings_web_new(void *box, uintptr_t handle) {
 WebKitWebView *view = WEBKIT_WEB_VIEW(webkit_web_view_new());
 g_signal_connect(view, "notify::title", G_CALLBACK(settings_title), (gpointer)handle);
 g_signal_connect(view, "decide-policy", G_CALLBACK(settings_policy), NULL);
 gtk_box_pack_start(GTK_BOX(box), GTK_WIDGET(view), TRUE, TRUE, 0);
 return view;
}
static void settings_eval(WebKitWebView *view, const char *script) {
 webkit_web_view_evaluate_javascript(view, script, -1, NULL, NULL, NULL, NULL, NULL);
}
*/
import "C"

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime/cgo"
	"strings"
	"time"
	"unsafe"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

//go:embed settings-web/index.html
var settingsHTML string

type webSettings struct {
	ui   *settingsUI
	view *C.WebKitWebView
}

func showSettings() {
	ui := &settingsUI{alive: true}
	ui.win, _ = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	ui.win.SetTitle("Einstellungen — MiracleOS")
	ui.win.SetDefaultSize(1160, 760)
	ui.win.SetSizeRequest(760, 540)
	root, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	root.PackStart(glassWindowHeader(ui.win, "Einstellungen — MiracleOS"), false, false, 0)
	ui.status = settingsText("") // Shared native dialogs retain their existing status target.
	ui.win.Add(root)
	web := &webSettings{ui: ui}
	handle := cgo.NewHandle(web)
	web.view = C.settings_web_new(unsafe.Pointer(root.GObject), C.uintptr_t(handle))
	ui.win.Connect("destroy", func() { ui.alive = false; handle.Delete(); gtk.MainQuit() })
	html := C.CString(settingsHTML)
	C.webkit_web_view_load_html(web.view, html, nil)
	C.free(unsafe.Pointer(html))
	ui.win.ShowAll()
	lastPoll := ""
	glib.TimeoutAdd(2000, func() bool {
		if !ui.alive {
			return false
		}
		preferences, _ := json.Marshal(readSystemPreferences())
		updates, _ := os.ReadFile("/var/lib/miracleos/updates/status.json")
		apps, _ := json.Marshal(installedApplications())
		poll := string(preferences) + string(updates) + string(apps)
		if poll != lastPoll {
			lastPoll = poll
			web.sendState(0, "")
		}
		return true
	})
}

func (web *webSettings) evaluate(script string) {
	if !web.ui.alive {
		return
	}
	s := C.CString(script)
	defer C.free(unsafe.Pointer(s))
	C.settings_eval(web.view, s)
}

func (web *webSettings) sendState(id int, message string) {
	name := os.Getenv("USER")
	if current, e := user.Current(); e == nil {
		name = current.Username
	}
	page := ""
	if len(os.Args) > 2 {
		page = os.Args[2]
	}
	apps := installedApplications()
	pins := map[string]bool{}
	for _, app := range apps {
		pins[app.ID] = pinnedApp(app.ID)
	}
	var update systemUpdateStatus
	raw, _ := os.ReadFile("/var/lib/miracleos/updates/status.json")
	_ = json.Unmarshal(raw, &update)
	p := readSystemPreferences()
	wallpaper := ""
	if b, e := os.ReadFile(p.Wallpaper); e == nil && len(b) <= 12*1024*1024 {
		wallpaper = "data:image/" + wallpaperMIME(b) + ";base64," + base64.StdEncoding.EncodeToString(b)
	}
	data, _ := json.Marshal(map[string]interface{}{"id": id, "message": message, "preferences": p, "user": name, "page": page, "apps": apps, "pins": pins, "update": update, "wallpaper": wallpaper})
	web.evaluate("window.receiveState && window.receiveState(" + string(data) + ")")
}

func wallpaperMIME(b []byte) string {
	if len(b) > 3 && string(b[:3]) == "GIF" {
		return "gif"
	}
	if len(b) > 2 && b[0] == 255 && b[1] == 216 {
		return "jpeg"
	}
	if len(b) > 12 && string(b[8:12]) == "WEBP" {
		return "webp"
	}
	return "png"
}

//export settingsWebCommand
func settingsWebCommand(handle C.uintptr_t, message *C.char) {
	text := C.GoString(message)
	if !strings.HasPrefix(text, "miracle-settings:") {
		return
	}
	web := cgo.Handle(handle).Value().(*webSettings)
	var request struct {
		ID     int             `json:"id"`
		Action string          `json:"action"`
		Key    string          `json:"key"`
		Value  json.RawMessage `json:"value"`
	}
	if json.Unmarshal([]byte(strings.TrimPrefix(text, "miracle-settings:")), &request) != nil {
		return
	}
	id := request.ID
	done := func(message string) { web.sendState(id, message) }
	p := readSystemPreferences()
	switch request.Action {
	case "ready":
		done("")
	case "save":
		valid := false
		switch request.Key {
		case "dark":
			valid = json.Unmarshal(request.Value, &p.Dark) == nil
		case "transparency":
			valid = json.Unmarshal(request.Value, &p.Transparency) == nil
		case "accent":
			valid = json.Unmarshal(request.Value, &p.Accent) == nil && regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(p.Accent)
		case "font_size":
			valid = json.Unmarshal(request.Value, &p.FontSize) == nil && (p.FontSize == 10 || p.FontSize == 12 || p.FontSize == 14)
		case "frame_color":
			valid = json.Unmarshal(request.Value, &p.FrameColor) == nil && validFrameColor(p.FrameColor)
		case "appearance_reset":
			p.Dark = false
			p.Accent = "#bf69a4"
			p.Transparency = true
			p.FontSize = 12
			valid = true
		case "wallpaper_reset":
			p.CustomWallpaper = false
			p.Wallpaper = ""
			valid = true
		}
		if !valid {
			done("Ungültige Einstellung")
			return
		}
		web.persist(p, done)
	case "app_frame":
		var color string
		if json.Unmarshal(request.Value, &color) != nil || !validFrameColor(color) {
			done("Ungültige Fensterfarbe")
			return
		}
		for _, app := range installedApplications() {
			if app.ID == request.Key {
				if p.AppFrames == nil {
					p.AppFrames = map[string]appFramePreference{}
				}
				if color == "" {
					delete(p.AppFrames, app.ID)
				} else {
					p.AppFrames[app.ID] = appFrameRule(app, color)
				}
				web.persist(p, done)
				return
			}
		}
		done("App nicht mehr verfügbar")
	case "wallpaper":
		showFilePicker(&filePickerOptions{Title: "Hintergrund auswählen", Parent: web.ui.win, Filters: []pickerFilter{{Name: "Bilder", Rules: []pickerRule{{Kind: 0, Value: "*.png"}, {Kind: 0, Value: "*.jpg"}, {Kind: 0, Value: "*.jpeg"}, {Kind: 0, Value: "*.webp"}, {Kind: 0, Value: "*.gif"}}}}, Done: func(paths []string) {
			if !web.ui.alive {
				return
			}
			if len(paths) == 0 {
				done("")
				return
			}
			if _, err := gdk.PixbufNewFromFile(paths[0]); err != nil {
				done("Dieses Bild kann nicht geöffnet werden.")
				return
			}
			current := readSystemPreferences()
			current.Wallpaper = paths[0]
			current.CustomWallpaper = true
			web.persist(current, done)
		}})
		// Cancelling the existing picker does not invoke Done. Release the bridge
		// now; a successful selection publishes fresh preferences separately.
		done("")
	case "password":
		web.ui.passwordDialog()
		done("")
	case "reset":
		web.ui.resetSystemDialog()
		done("")
	case "update":
		if request.Key == "apply" || request.Key == "enable" || request.Key == "disable" || request.Key == "rollback" {
			web.ui.updateActionDialog(request.Key)
		}
		done("")
	case "installer":
		exe, _ := os.Executable()
		err := exec.Command(exe, "--install").Start()
		if err != nil {
			done(err.Error())
		} else {
			done("")
		}
	case "launch", "pin", "uninstall":
		for _, app := range installedApplications() {
			if app.ID == request.Key {
				switch request.Action {
				case "launch":
					launchInstalledApp(app)
				case "pin":
					if err := pinApp(app, !pinnedApp(app.ID)); err != nil {
						done("Anheften fehlgeschlagen: " + err.Error())
						return
					}
				case "uninstall":
					uninstallApp(app)
				}
				break
			}
		}
		done("")
	case "info":
		go func() {
			result := webSettingsInformation(request.Key)
			glib.IdleAdd(func() {
				if !web.ui.alive {
					return
				}
				data, _ := json.Marshal(map[string]interface{}{"id": id, "page": request.Key, "text": result})
				web.evaluate("window.receiveInfo(" + string(data) + ")")
			})
		}()
	default:
		done("Unbekannte Aktion")
	}
}

func validFrameColor(color string) bool {
	for _, choice := range framePalette {
		if choice.color == color {
			return true
		}
	}
	return false
}
func (web *webSettings) persist(p systemPreferences, done func(string)) {
	previous := readSystemPreferences().Wallpaper
	p = modeWallpaper(p)
	if err := saveSystemPreferences(p); err != nil {
		done("Speichern fehlgeschlagen: " + err.Error())
		return
	}
	if previous != p.Wallpaper {
		if err := applyWallpaper(p.Wallpaper); err != nil {
			done("Gespeichert; Hintergrund konnte nicht angewendet werden.")
			return
		}
	}
	done("Gespeichert · systemweit übernommen")
}
func webSettingsInformation(page string) string {
	switch page {
	case "System":
		host, _ := os.Hostname()
		return "MiracleOS Desktop\nDebian GNU/Linux\n\nGerät: " + host + "\nKernel: " + settingsCommand("uname", "-r") + "\nArchitektur: " + settingsCommand("uname", "-m")
	case "Netzwerk":
		return settingsCommand("nmcli", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device") + "\n\nIn virtuellen Maschinen sind nur die durchgereichten Netzwerkadapter verfügbar."
	case "Geräte":
		return "USB-Geräte\n" + settingsCommand("lsusb") + "\n\nBluetooth\n" + settingsCommand("bluetoothctl", "list")
	case "Konten":
		if current, e := user.Current(); e == nil {
			return "Benutzer: " + current.Username + "\nPersönlicher Ordner: " + current.HomeDir + "\n\nLokales Debian-Konto"
		}
	case "Zeit & Sprache":
		return time.Now().Format("02.01.2006 · 15:04") + "\n\n" + settingsCommand("timedatectl", "status")
	case "Datenschutz":
		return "Deine Einstellungen bleiben lokal.\n\nDarstellung und Systemfarben werden in deinem Benutzerordner gespeichert. Diese Einstellungs-App sendet keine Nutzungsdaten. Installierte Programme haben eigene Datenschutzoptionen."
	}
	return fmt.Sprintf("Informationen für %s sind nicht verfügbar.", page)
}
