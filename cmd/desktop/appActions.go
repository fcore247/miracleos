package main

import (
	"fmt"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"os/exec"
	"regexp"
	"strings"
)

func appContextActions(app installedApp) []contextAction {
	pinLabel := "An die Leiste anheften"
	pin := !pinnedApp(app.ID)
	if !pin {
		pinLabel = "Von der Leiste lösen"
	}
	return []contextAction{{"Öffnen", func() { launchInstalledApp(app) }, false}, {pinLabel, func() { _ = pinApp(app, pin) }, false}, {"Deinstallieren …", func() { uninstallApp(app) }, false}}
}
func attachAppContext(w gtk.IWidget, owner *gtk.Window, app installedApp) {
	widget := w.ToWidget()
	widget.AddEvents(int(gdk.BUTTON_PRESS_MASK))
	widget.Connect("button-press-event", func(_ interface{}, event *gdk.Event) bool {
		if gdk.EventButtonNewFromEvent(event).Button() != 3 {
			return false
		}
		glassContextMenu(owner, event, appContextActions(app))
		return true
	})
}
func uninstallApp(app installedApp) {
	// The start layer owns exclusive keyboard focus and sits above normal windows.
	// Unmap it before showing a modal dialog, and let GTK release the context
	// menu's grab before continuing with the uninstall flow.
	hideStartMenu()
	glib.IdleAdd(func() { beginAppUninstall(app) })
}
func beginAppUninstall(app installedApp) {
	if prefix, ok := windowsLauncherPrefix(app.File); ok {
		uninstallWindowsApp(app, prefix)
		return
	}
	// Resolve the owning package, then show APT's complete removal plan before asking
	// for authentication. No shell interpolation or automatic dependency removal.
	go func() {
		out, err := exec.Command("dpkg-query", "-S", app.File).Output()
		pkg := ""
		if err == nil {
			pkg = strings.SplitN(string(out), ": ", 2)[0]
		}
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*(?::[a-z0-9]+)?$`).MatchString(pkg) {
			glib.IdleAdd(func() {
				appActionMessage("Diese App gehört zu keinem erkannten Debian-Paket. Bitte verwende ihren ursprünglichen Paketmanager.")
			})
			return
		}
		plan, e := exec.Command("apt-get", "--simulate", "remove", "--", pkg).CombinedOutput()
		glib.IdleAdd(func() {
			if e != nil {
				appActionMessage(string(plan))
				return
			}
			dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_OK_CANCEL, "%s deinstallieren?\n\n%s", app.Name, string(plan))
			dialog.SetTitle("App deinstallieren — MiracleOS")
			setUninstallerIcon(&dialog.Window)
			dialog.Connect("response", func(_ *gtk.Dialog, response gtk.ResponseType) {
				dialog.Destroy()
				if response != gtk.RESPONSE_OK {
					return
				}
				go func() {
					result, e := exec.Command("pkexec", "/usr/bin/apt-get", "-y", "remove", "--", pkg).CombinedOutput()
					glib.IdleAdd(func() {
						if e != nil {
							appActionMessage("Deinstallation nicht abgeschlossen:\n" + string(result))
						} else {
							_ = pinApp(app, false)
							appActionMessage(app.Name + " wurde deinstalliert.")
						}
					})
				}()
			})
			dialog.ShowAll()
		})
	}()
}
func appActionMessage(text string) { appActionDialog("Deinstallation — MiracleOS", text, nil) }

func setUninstallerIcon(win *gtk.Window) {
	setUtilityWindowIcon(win, "uninstall")
}

func setUtilityWindowIcon(win *gtk.Window, name string) {
	if data, err := settingsIconFiles.ReadFile("images/settings-icons/" + name + ".png"); err == nil {
		if icon, err := gdk.PixbufNewFromBytesOnly(data); err == nil {
			win.SetIcon(icon)
		}
	}
}

func appActionDialog(title, text string, confirm func()) {
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	win.SetTitle(title)
	setUninstallerIcon(win)
	win.SetDefaultSize(540, 290)
	if confirm != nil {
		win.SetModal(true)
	}
	root, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	menuClass(root, "fm-shell")
	root.PackStart(glassWindowHeader(win, title), false, false, 0)
	body, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 16)
	menuClass(body, "installer-content")
	body.PackStart(settingIcon("miracleos-uninstall", 64), false, false, 0)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scroll.SetMinContentHeight(100)
	scroll.Add(settingsText(text))
	body.PackStart(scroll, true, true, 0)
	buttons, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	label := "Schließen"
	if confirm != nil {
		label = "Abbrechen"
	}
	buttons.PackStart(settingsButton(label, func() { win.Destroy() }), false, false, 0)
	if confirm != nil {
		button := settingsButton("Deinstallation öffnen", func() { win.Destroy(); confirm() })
		menuClass(button, "installer-primary")
		buttons.PackEnd(button, false, false, 0)
	}
	body.PackEnd(buttons, false, false, 0)
	root.PackStart(body, true, true, 0)
	win.Add(root)
	win.ShowAll()
}

var _ = fmt.Sprintf
