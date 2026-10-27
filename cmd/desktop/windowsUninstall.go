package main

/*
#cgo pkg-config: glib-2.0
#include <glib.h>
#include <string.h>
#include <stdlib.h>
// Parse desktop command arguments as data; never execute a launcher in a shell.
static char *windows_launcher_prefix(const char *line) {
 int argc = 0; char **argv = NULL;
 if (!g_shell_parse_argv(line, &argc, &argv, NULL) || !argc) return NULL;
 int i = 0; const char *prefix = "";
 char *base = g_path_get_basename(argv[0]);
 if (!strcmp(base,"env")) {
  i = 1;
  for (; i < argc && strchr(argv[i],'='); i++)
   if (g_str_has_prefix(argv[i],"WINEPREFIX=")) prefix = argv[i]+11;
 }
 g_free(base);
 char *result = NULL;
 if (i < argc) {
  base = g_path_get_basename(argv[i]);
  if (!strcmp(base,"wine") || !strcmp(base,"wine-stable") || !strcmp(base,"wine64") || !strcmp(base,"wine64-stable")) result = g_strdup(prefix);
  g_free(base);
 }
 g_strfreev(argv);
 return result;
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"github.com/gotk3/gotk3/glib"
	"gopkg.in/ini.v1"
)

func windowsLauncherPrefix(file string) (string, bool) {
	data, err := ini.Load(file)
	if err != nil {
		return "", false
	}
	line := C.CString(data.Section("Desktop Entry").Key("Exec").String())
	defer C.free(unsafe.Pointer(line))
	pointer := C.windows_launcher_prefix(line)
	if pointer == nil {
		return "", false
	}
	defer C.g_free(C.gpointer(pointer))
	prefix := C.GoString(pointer)
	if prefix == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		prefix = filepath.Join(home, ".wine")
	}
	if !filepath.IsAbs(prefix) {
		return "", false
	}
	return filepath.Clean(prefix), true
}

func wineUninstallEntries(prefix string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wine", "uninstaller", "--list")
	cmd.Env = windowsEnvironment(prefix)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func windowsUninstallKey(entries, name string) string {
	name = strings.TrimSpace(name)
	if strings.EqualFold(filepath.Ext(name), ".exe") {
		name = name[:len(name)-4]
	}
	architecture := regexp.MustCompile(`(?i)\s+\((?:64-bit x64|32-bit x86|64-bit|32-bit|x64|x86)\)$`)
	match := ""
	for _, line := range strings.Split(entries, "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), "|||", 2)
		if len(fields) != 2 {
			continue
		}
		display := architecture.ReplaceAllString(strings.TrimSpace(fields[1]), "")
		if strings.EqualFold(display, name) {
			if match != "" && match != fields[0] {
				return ""
			} // Ambiguous: show Wine's program chooser.
			match = fields[0]
		}
	}
	return match
}

func wineEntryPresent(entries, key string) bool {
	for _, line := range strings.Split(entries, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), key+"|||") {
			return true
		}
	}
	return false
}

func removeWindowsLauncher(app installedApp, prefix string) error {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		base = filepath.Join(home, ".local/share")
	}
	root := filepath.Join(base, "applications")
	fileRel, err := filepath.Rel(root, app.File)
	if err != nil || fileRel == ".." || strings.HasPrefix(fileRel, ".."+string(os.PathSeparator)) {
		return nil
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(app.File))
	if err != nil {
		if os.IsNotExist(err) {
			return pinApp(app, false)
		}
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, parent)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil
	}
	if _, err = os.Lstat(app.File); os.IsNotExist(err) {
		return pinApp(app, false)
	}
	if actual, ok := windowsLauncherPrefix(app.File); !ok || actual != prefix {
		return fmt.Errorf("Der App-Starter wurde während der Deinstallation geändert.")
	}
	if err = os.Remove(app.File); err != nil && !os.IsNotExist(err) {
		return err
	}
	return pinApp(app, false)
}

func uninstallWindowsApp(app installedApp, prefix string) {
	go func() {
		entries, err := wineUninstallEntries(prefix)
		key := windowsUninstallKey(entries, app.Name)
		glib.IdleAdd(func() {
			if err != nil {
				appActionMessage("Die Windows-Deinstallation konnte nicht geöffnet werden. Bitte die Windows-App schließen und erneut versuchen.")
				return
			}
			message := "Die Windows-Deinstallation für " + app.Name + " wird geöffnet. Bestätige die Entfernung dort."
			if key == "" {
				message = "Die Windows-Programmverwaltung wird geöffnet. Wähle dort " + app.Name + " aus und klicke auf Entfernen."
			}
			appActionDialog("Windows-App deinstallieren — MiracleOS", message, func() {
				go func() {
					args := []string{"uninstaller"}
					if key != "" {
						args = append(args, "--remove", key)
					}
					cmd := exec.Command("wine", args...)
					cmd.Env = windowsEnvironment(prefix)
					output, runErr := cmd.CombinedOutput()
					after, checkErr := wineUninstallEntries(prefix)
					removed := key != "" && checkErr == nil && !wineEntryPresent(after, key)
					var cleanErr error
					if removed {
						cleanErr = removeWindowsLauncher(app, prefix)
					}
					glib.IdleAdd(func() {
						if removed {
							if cleanErr != nil {
								appActionMessage(app.Name + " wurde entfernt. Der Starter konnte nicht vollständig aufgeräumt werden.")
							} else {
								appActionMessage(app.Name + " wurde deinstalliert.")
							}
							return
						}
						if runErr != nil && key != "" {
							appActionMessage(fmt.Sprintf("Die Deinstallation wurde nicht abgeschlossen.\n%s", strings.TrimSpace(string(output))))
							return
						}
						if key != "" {
							appActionMessage("Die App ist weiterhin installiert. Die Deinstallation wurde abgebrochen oder nicht abgeschlossen.")
						}
					})
				}()
			})
		})
	}()
}
