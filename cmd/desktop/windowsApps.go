package main

import (
	"context"
	"debug/pe"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

var windowsMIMEs = []string{"application/x-ms-dos-executable", "application/x-msdownload", "application/vnd.microsoft.portable-executable", "application/x-dosexec", "application/x-msi", "application/x-ms-installer"}

func windowsAppPath(path string) (string, error) {
	if strings.HasPrefix(path, "file:") {
		u, err := url.Parse(path)
		if err != nil || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("Bitte eine lokale Windows-App auswählen.")
		}
		path = u.Path
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".exe" && ext != ".msi" {
		return "", fmt.Errorf("Bitte eine Windows-App (.exe) oder einen Windows-Installer (.msi) auswählen.")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Diese Datei ist nicht verfügbar.")
	}
	if ext == ".exe" {
		file, err := pe.Open(path)
		if err != nil {
			return "", fmt.Errorf("Diese Datei ist keine gültige Windows-App.")
		}
		defer file.Close()
		if file.Machine != pe.IMAGE_FILE_MACHINE_I386 && file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return "", fmt.Errorf("Diese Windows-App benötigt eine andere Prozessorarchitektur.")
		}
	} else {
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		var header [8]byte
		if _, err = io.ReadFull(file, header[:]); err != nil || string(header[:]) != "\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1" {
			return "", fmt.Errorf("Diese Datei ist kein gültiger Windows-Installer.")
		}
	}
	return path, nil
}

func windowsPrefix() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local/share")
	}
	return filepath.Join(base, "miracleos", "windows")
}

func windowsEnvironment(prefix string) []string {
	result := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "WINEPREFIX=") && !strings.HasPrefix(entry, "WINEDEBUG=") && !strings.HasPrefix(entry, "WINEARCH=") {
			result = append(result, entry)
		}
	}
	return append(result, "WINEPREFIX="+prefix, "WINEDEBUG=-all")
}

func prepareWindowsEnvironment(prefix string, log *os.File) error {
	if err := os.MkdirAll(prefix, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(prefix, ".miracleos-init.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ready := filepath.Join(prefix, ".miracleos-ready")
	if _, err = os.Stat(ready); err == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wineboot", "--init")
	cmd.Env = windowsEnvironment(prefix)
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("Die Windows-Umgebung konnte nicht vorbereitet werden: %w", err)
	}
	return os.WriteFile(ready, []byte("1\n"), 0600)
}

func windowsCommand(path, prefix string) *exec.Cmd {
	args := []string{path}
	if strings.EqualFold(filepath.Ext(path), ".msi") {
		args = []string{"msiexec", "/i", path}
	}
	cmd := exec.Command("wine", args...)
	cmd.Env = windowsEnvironment(prefix)
	cmd.Dir = filepath.Dir(path)
	return cmd
}

func windowsSupportInstalled() bool {
	if _, err := exec.LookPath("wine"); err != nil {
		return false
	}
	out, err := exec.Command("dpkg-query", "-W", "-f=${Status}\n", "wine64", "wine32:i386").Output()
	states := strings.Split(strings.TrimSpace(string(out)), "\n")
	return err == nil && len(states) == 2 && states[0] == "install ok installed" && states[1] == "install ok installed"
}

// Existing signed component updates need no new root helper or updater format.
// First use installs only this fixed Debian package set after GUI authorization.
func installWindowsSupport(password string, log *os.File) error {
	if password == "" || strings.ContainsAny(password, "\r\n\x00") {
		return fmt.Errorf("Bitte dein MiracleOS-Passwort eingeben.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	commands := [][]string{
		{"/usr/bin/dpkg", "--add-architecture", "i386"},
		{"/usr/bin/apt-get", "update"},
		{"/usr/bin/apt-get", "--assume-yes", "--no-remove", "-o", "DPkg::Lock::Timeout=120", "-o", "Dpkg::Use-Pty=0", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "install", "wine", "wine64", "wine32:i386", "fonts-wine"},
	}
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, "sudo", append([]string{"-S", "-k", "-p", ""}, args...)...)
		cmd.Stdin = strings.NewReader(password + "\n")
		cmd.Stdout = log
		cmd.Stderr = log
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Windows-Unterstützung konnte nicht installiert werden. Prüfe dein Passwort und die Internetverbindung.")
		}
	}
	if !windowsSupportInstalled() {
		return fmt.Errorf("Die Windows-Unterstützung wurde nicht vollständig installiert.")
	}
	return nil
}

func showWindowsApp(path string) {
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	win.SetTitle("Windows-App — MiracleOS")
	win.SetDefaultSize(520, 340)
	root, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	menuClass(root, "fm-shell")
	root.PackStart(glassWindowHeader(win, "Windows-App — MiracleOS"), false, false, 0)
	body, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 16)
	menuClass(body, "installer-content")
	body.PackStart(menuLabel("Das ist eine Windows-App.", "installer-title"), false, false, 0)
	body.PackStart(settingsText("Kein Problem – ich kann versuchen, sie mit Wine auszuführen. Die Einrichtung erledige ich für dich. Manche Windows-Programme sind nicht kompatibel."), false, false, 0)
	body.PackStart(menuLabel(filepath.Base(path), "settings-card-title"), false, false, 0)
	status := settingsText("Bereit zum Starten.")
	body.PackStart(status, false, false, 0)
	password, _ := gtk.EntryNew()
	password.SetVisibility(false)
	password.SetInputPurpose(gtk.INPUT_PURPOSE_PASSWORD)
	password.SetPlaceholderText("MiracleOS-Passwort für die einmalige Einrichtung")
	needsSupport := !windowsSupportInstalled()
	password.SetNoShowAll(!needsSupport)
	body.PackStart(password, false, false, 0)
	if needsSupport {
		status.SetText("Ich richte die Windows-Unterstützung automatisch ein. Bestätige das einmalig mit deinem Passwort; eine Internetverbindung ist erforderlich.")
	}
	buttons, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	start, _ := gtk.ButtonNewWithLabel("Mit Wine starten")
	menuClass(start, "installer-primary")
	buttons.PackStart(start, false, false, 0)
	buttons.PackEnd(settingsButton("Schließen", func() { win.Destroy() }), false, false, 0)
	body.PackEnd(buttons, false, false, 0)
	root.PackStart(body, true, true, 0)
	win.Add(root)
	alive, busy := true, false
	win.Connect("destroy", func() {
		alive = false
		if !busy {
			gtk.MainQuit()
		}
	})
	validated, err := windowsAppPath(path)
	if err != nil {
		status.SetText(err.Error())
		start.SetSensitive(false)
	}
	start.Connect("clicked", func() {
		if busy {
			return
		}
		secret, _ := password.GetText()
		if needsSupport && (secret == "" || strings.ContainsAny(secret, "\r\n\x00")) {
			status.SetText("Bitte dein Passwort für die einmalige Einrichtung eingeben.")
			return
		}
		password.SetText("")
		password.SetSensitive(false)
		busy = true
		start.SetSensitive(false)
		status.SetText("Windows-Umgebung wird vorbereitet. Beim ersten Start dauert das einen Moment …")
		go func() {
			prefix := windowsPrefix()
			err := os.MkdirAll(prefix, 0700)
			var log *os.File
			if err == nil {
				log, err = os.OpenFile(filepath.Join(prefix, "last-launch.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			}
			if err == nil {
				defer log.Close()
				if needsSupport {
					err = installWindowsSupport(secret, log)
				}
			}
			secret = ""
			if err == nil {
				err = prepareWindowsEnvironment(prefix, log)
			}
			if err == nil {
				glib.IdleAdd(func() {
					if alive {
						status.SetText("Die Windows-App läuft. Du kannst dieses Hinweisfenster schließen.")
					}
				})
				cmd := windowsCommand(validated, prefix)
				cmd.Stdout = log
				cmd.Stderr = log
				err = cmd.Run()
			}
			glib.IdleAdd(func() {
				busy = false
				if !alive {
					gtk.MainQuit()
					return
				}
				needsSupport = !windowsSupportInstalled()
				password.SetSensitive(true)
				if !needsSupport {
					password.Hide()
				}
				start.SetSensitive(true)
				start.SetLabel("Erneut starten")
				if err != nil {
					status.SetText("Start nicht abgeschlossen: " + err.Error() + " Details stehen im Windows-Startprotokoll.")
				} else {
					win.Destroy()
				}
			})
		}()
	})
	win.ShowAll()
}

func ensureWindowsLauncher() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		base = filepath.Join(home, ".local/share")
	}
	dir := filepath.Join(base, "applications")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(exe)
	data := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Windows-Apps\nComment=Windows-Programme mit Wine öffnen\nExec=\"%s\" --windows %%f\nIcon=wine\nTerminal=false\nNoDisplay=true\nMimeType=%s;\nStartupWMClass=miracleos-windows\n", escaped, strings.Join(windowsMIMEs, ";"))
	path := filepath.Join(dir, "miracleos-windows.desktop")
	if previous, e := os.ReadFile(path); e == nil && string(previous) == data {
		return nil
	}
	if err = os.WriteFile(path, []byte(data), 0644); err != nil {
		return err
	}
	_ = exec.Command("update-desktop-database", dir).Run()
	for _, mime := range windowsMIMEs {
		_ = exec.Command("xdg-mime", "default", "miracleos-windows.desktop", mime).Run()
	}
	return nil
}
