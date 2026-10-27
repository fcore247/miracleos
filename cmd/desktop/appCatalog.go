package main

/*
#cgo pkg-config: gio-unix-2.0
#include <gio/gdesktopappinfo.h>
#include <stdlib.h>
static GAppInfo *catalog_info(GList *entry) { return G_APP_INFO(entry->data); }
static const char *catalog_file(GAppInfo *info) { return G_IS_DESKTOP_APP_INFO(info) ? g_desktop_app_info_get_filename(G_DESKTOP_APP_INFO(info)) : NULL; }
static const char *catalog_wmclass(GAppInfo *info) { return G_IS_DESKTOP_APP_INFO(info) ? g_desktop_app_info_get_startup_wm_class(G_DESKTOP_APP_INFO(info)) : NULL; }
*/
import "C"
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type installedApp struct{ ID, Name, Description, Icon, File, WMClass string }

func installedApplications() []installedApp {
	list := C.g_app_info_get_all()
	defer C.g_list_free(list)
	apps := []installedApp{}
	for entry := list; entry != nil; entry = entry.next {
		info := C.catalog_info(entry)
		if C.g_app_info_should_show(info) != 0 {
			app := installedApp{ID: C.GoString(C.g_app_info_get_id(info)), Name: C.GoString(C.g_app_info_get_display_name(info)), Description: C.GoString(C.g_app_info_get_description(info)), File: C.GoString(C.catalog_file(info)), WMClass: C.GoString(C.catalog_wmclass(info))}
			if icon := C.g_app_info_get_icon(info); icon != nil {
				s := C.g_icon_to_string(icon)
				app.Icon = C.GoString(s)
				C.g_free(C.gpointer(s))
			}
			if app.Name != "" && app.File != "" {
				apps = append(apps, app)
			}
		}
		C.g_object_unref(C.gpointer(info))
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name) })
	return apps
}
func launchInstalledApp(app installedApp) {
	hideStartMenu()
	go func() {
		cmd := exec.Command("gio", "launch", app.File)
		if output, err := cmd.CombinedOutput(); err != nil {
			glib.IdleAdd(func() {
				dialog := gtk.MessageDialogNew(nil, 0, gtk.MESSAGE_ERROR, gtk.BUTTONS_CLOSE, "%s konnte nicht gestartet werden.\n%s", app.Name, strings.TrimSpace(string(output)))
				dialog.Connect("response", func() { dialog.Destroy() })
				dialog.ShowAll()
			})
		}
	}()
}
func installedAppIcon(app installedApp) *gtk.Image {
	if app.Name == "Paketinstaller" { return settingIcon("miracleos-package-installer", 24) }
 if app.Name == "Einstellungen" { return settingIcon("preferences-system",24) }
	if app.Name == "Dateien" {
		return menuIcon("files", 24)
	}
	icon := app.Icon
	if strings.HasPrefix(icon, "file://") {
		icon = strings.TrimPrefix(icon, "file://")
	}
	// These icons are embedded so installed binaries and updates do not need
	// a source checkout beside the executable to render their taskbar icons.
	if filepath.Base(filepath.Dir(icon)) == "settings-icons" {
		if data, err := settingsIconFiles.ReadFile("images/settings-icons/" + filepath.Base(icon)); err == nil {
			if pixbuf, err := gdk.PixbufNewFromBytesOnly(data); err == nil {
				image, _ := gtk.ImageNewFromPixbuf(scalePixbuf(pixbuf, 24, 24))
				return image
			}
		}
	}
	if filepath.IsAbs(icon) {
		if pixbuf, err := gdk.PixbufNewFromFileAtScale(icon, 24, 24, true); err == nil {
			image, _ := gtk.ImageNewFromPixbuf(pixbuf)
			return image
		}
		icon = "application-x-executable"
	}
	if icon == "" || strings.HasPrefix(icon, ".") {
		icon = "application-x-executable"
	}
	image, _ := gtk.ImageNewFromIconName(strings.Fields(icon)[0], gtk.ICON_SIZE_LARGE_TOOLBAR)
	image.SetPixelSize(24)
	return image
}

// Start menu and settings share one live catalog, including user and exported apps.
func installedAppsView(owner *gtk.Window, search *gtk.Entry, settings ...*settingsUI) *gtk.ScrolledWindow {
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	scroll.Add(box)
	var alive atomic.Bool
	alive.Store(true)
	scroll.Connect("destroy", func() { alive.Store(false) })
	var apps []installedApp
	render := func() {
		box.GetChildren().Foreach(func(child interface{}) {
			if widget, ok := child.(gtk.IWidget); ok {
				widget.ToWidget().Destroy()
			}
		})
		query := ""
		if search != nil {
			query, _ = search.GetText()
		}
		query = strings.ToLower(strings.TrimSpace(query))
		count := 0
		for _, app := range apps {
 if len(settings)==0 && hiddenFromStart(app.ID) { continue }
			if query != "" && !strings.Contains(strings.ToLower(app.Name+" "+app.Description), query) {
				continue
			}
			count++
			button, _ := gtk.ButtonNew()
			menuClass(button, "fm-place")
			row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
			row.PackStart(installedAppIcon(app), false, false, 0)
			label := menuLabel(app.Name, "fm-place-label")
			label.SetXAlign(0)
			label.SetLineWrap(true)
			row.PackStart(label, true, true, 0)
			button.Add(row)
			button.SetTooltipText(app.Description)
 layoutDragSource(button,layoutDrag{AppID:app.ID})
 attachAppContext(button,owner,app)
			app := app
			button.Connect("clicked", func() { launchInstalledApp(app) })
			if len(settings) > 0 {
				item, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
				item.PackStart(button, true, true, 0)
				item.PackEnd(appFramePicker(app, settings[0]), false, false, 0)
				box.PackStart(item, false, false, 0)
			} else {
				box.PackStart(button, false, false, 0)
			}
		}
		if count == 0 {
			box.PackStart(menuLabel("Keine passenden Anwendungen", "fm-muted"), false, false, 8)
		}
		box.ShowAll()
	}
	busy := false
	refresh := func() bool {
		if !alive.Load() {
			return false
		}
		if busy {
			return true
		}
		busy = true
		go func() {
			next := installedApplications()
			glib.IdleAdd(func() {
				busy = false
				if !alive.Load() {
					return
				}
				if !reflect.DeepEqual(apps, next) {
					apps = next
					render()
				}
			})
		}()
		return true
	}
	if search != nil {
		search.Connect("changed", func() {
			if alive.Load() {
				render()
			}
		})
	}
	box.PackStart(menuLabel("Anwendungen werden geladen …", "fm-muted"), false, false, 8)
	owner.Connect("show", func() { refresh() })
	refresh()
	glib.TimeoutAdd(3000, refresh)
	return scroll
}
func ensureTerminalLauncher() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".local/share")
	}
	dir = filepath.Join(dir, "applications")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(exe)
	return os.WriteFile(filepath.Join(dir, "miracleos-terminal.desktop"), []byte(fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Terminal\nComment=Echtes Debian-Terminal\nExec=\"%s\" --terminal\nIcon=utilities-terminal\nTerminal=false\nCategories=System;TerminalEmulator;\n", escaped)), 0644)
}

func hiddenFromStart(id string) bool {
 switch id {case "foot.desktop","footclient.desktop","foot-server.desktop","gdebi.desktop":return true};return false
}
