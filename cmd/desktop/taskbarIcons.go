package main

import (
	"os"
	"path/filepath"
	"strings"
)

func taskbarAppIcon(appID, title string, apps []installedApp) string {
	exe, _ := os.Executable()
	images := filepath.Join(filepath.Dir(exe), "cmd", "desktop", "images")
	switch {
	case strings.HasPrefix(title, "Windows-App deinstallieren — MiracleOS"), strings.HasPrefix(title, "App deinstallieren — MiracleOS"), strings.HasPrefix(title, "Deinstallation — MiracleOS"):
		return filepath.Join(images, "settings-icons", "uninstall.png")
	case strings.HasPrefix(title, "Terminal — MiracleOS"):
		return "utilities-terminal"
	case strings.HasPrefix(title, "Dateien — MiracleOS"), strings.HasPrefix(title, "Dateiauswahl — MiracleOS"), strings.HasPrefix(title, "Bild — "):
		return filepath.Join(images, "menu-icons", "files.png")
	case strings.HasPrefix(title, "Einstellungen — MiracleOS"):
		return filepath.Join(images, "settings-icons", "settings.png")
	case strings.HasPrefix(title, "Paketinstaller — MiracleOS"):
		return filepath.Join(images, "settings-icons", "package-installer.png")
	}
	id := strings.ToLower(strings.TrimSuffix(appID, ".desktop"))
	for _, app := range apps {
		if id != "" && (id == strings.ToLower(strings.TrimSuffix(app.ID, ".desktop")) || id == strings.ToLower(app.WMClass)) {
			return app.Icon
		}
	}
	if id == "foot" || id == "miracleos-terminal" {
		return "utilities-terminal"
	}
	return "application-x-executable"
}
