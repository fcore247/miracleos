package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWineUninstallRuntime(t *testing.T) {
	if os.Getenv("MIRACLEOS_WINE_TEST") != "1" {
		t.Skip("enable on the Wine test VM")
	}
	prefix := filepath.Join(t.TempDir(), "windows")
	log, err := os.CreateTemp("", "wine-uninstall-smoke-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	defer os.Remove(log.Name())
	if err = prepareWindowsEnvironment(prefix, log); err != nil {
		t.Fatal(err)
	}
	defer exec.Command("env", "WINEPREFIX="+prefix, "wineserver", "-k").Run()
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "wine", args...)
		cmd.Env = windowsEnvironment(prefix)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Wine %v: %v: %s", args, err, out)
		}
	}
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\MiracleOSUninstallTest`
	run("reg", "add", key, "/v", "DisplayName", "/d", "MiracleOS Uninstall Test", "/f")
	run("reg", "add", key, "/v", "UninstallString", "/d", `reg.exe delete "`+key+`" /f`, "/f")
	before, err := wineUninstallEntries(prefix)
	if err != nil {
		t.Fatal(err)
	}
	selected := windowsUninstallKey(before, "MiracleOS Uninstall Test")
	if selected != "MiracleOSUninstallTest" {
		t.Fatalf("fixture not found: %q", before)
	}
	run("uninstaller", "--remove", selected)
	after, err := wineUninstallEntries(prefix)
	if err != nil {
		t.Fatal(err)
	}
	if wineEntryPresent(after, selected) {
		t.Fatalf("uninstall did not remove fixture: %q", after)
	}
}

func TestWindowsLauncherPrefix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, test := range []struct {
		exec, prefix string
		wine         bool
	}{
		{`env WINEPREFIX="/home/user/Windows Apps" wine-stable C:\\\\Apps\\\\Editor.exe`, "/home/user/Windows Apps", true},
		{`env LANG=de_DE.UTF-8 WINEPREFIX=/tmp/windows /usr/bin/wine app.exe`, "/tmp/windows", true},
		{`wine app.exe`, filepath.Join(os.Getenv("HOME"), ".wine"), true},
		{`env WINEPREFIX=/tmp/windows /usr/bin/echo wine`, "", false},
		{`/opt/miracleos/miracleos-desktop --settings`, "", false},
		{`env WINEPREFIX=relative wine app.exe`, "", false},
	} {
		file := filepath.Join(t.TempDir(), "app.desktop")
		if err := os.WriteFile(file, []byte("[Desktop Entry]\nType=Application\nName=Test\nExec="+test.exec+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		prefix, ok := windowsLauncherPrefix(file)
		if prefix != test.prefix || ok != test.wine {
			t.Fatalf("%s: %q %v", test.exec, prefix, ok)
		}
	}
}

func TestWindowsUninstallMatchesUniqueProgram(t *testing.T) {
	entries := "Other|||Other program\nNotepad++|||Notepad++ (64-bit x64)\n"
	if key := windowsUninstallKey(entries, "Notepad++"); key != "Notepad++" {
		t.Fatal(key)
	}
	if key := windowsUninstallKey(entries, "notepad++.exe"); key != "Notepad++" {
		t.Fatal(key)
	}
	if key := windowsUninstallKey(entries, "Note"); key != "" {
		t.Fatal("partial match selected wrong program")
	}
	if key := windowsUninstallKey(entries+"Duplicate|||Notepad++ (32-bit x86)\n", "Notepad++"); key != "" {
		t.Fatal("ambiguous match selected a program")
	}
	if wineEntryPresent(entries, "Notepad") || !wineEntryPresent(entries, "Notepad++") {
		t.Fatal("incorrect uninstall verification")
	}
}

func TestWindowsUninstallCleansOnlySelectedLauncher(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	dir := filepath.Join(home, ".local/share/applications/wine/Programs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(home, "windows")
	file := filepath.Join(dir, "test.desktop")
	data := []byte("[Desktop Entry]\nName=Test\nExec=env WINEPREFIX=\"" + prefix + "\" wine app.exe\n")
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	app := installedApp{ID: "wine-Programs-test.desktop", Name: "Test", File: file}
	if err := pinApp(app, true); err != nil {
		t.Fatal(err)
	}
	if err := removeWindowsLauncher(app, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("starter remained")
	}
	if pinnedApp(app.ID) {
		t.Fatal("pin remained")
	}
	// Wine may remove the desktop file itself; a stale pin must still be removed.
	if err := pinApp(app, true); err != nil {
		t.Fatal(err)
	}
	if err := removeWindowsLauncher(app, prefix); err != nil {
		t.Fatal(err)
	}
	if pinnedApp(app.ID) {
		t.Fatal("pin for already removed starter remained")
	}
	outside := filepath.Join(home, "other.desktop")
	_ = os.WriteFile(outside, data, 0644)
	app.File = outside
	if err := removeWindowsLauncher(app, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("deleted an unrelated file outside applications")
	}
}
