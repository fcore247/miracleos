package main

import (
	"runtime"
	"testing"

	"github.com/gotk3/gotk3/gtk"
)

func TestStartMenuUninstallReleasesOverlay(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	gtk.Init(nil)
	menu, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		t.Fatal(err)
	}
	previous := activeMainMenu
	activeMainMenu = menu
	defer func() { activeMainMenu = previous; menu.Destroy() }()
	menu.ShowAll()
	if !menu.GetVisible() {
		t.Fatal("test start menu is not visible")
	}
	// Exercise the same action used by the start menu's right-click menu.
	actions := appContextActions(installedApp{ID: "uninstall-handoff-test", File: "/nonexistent/miracleos-uninstall-test.desktop", Name: "Test"})
	actions[len(actions)-1].run()
	if menu.GetVisible() {
		t.Fatal("uninstall left the start overlay above its confirmation dialog")
	}
}
