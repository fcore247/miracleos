package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type debPackage struct {
	Path, Name, Version, Architecture, Description, InstalledVersion string
	Size                                                             int64
}

func debPackagePath(path string) (string, error) {
	if strings.HasPrefix(path, "file:") {
		u, err := url.Parse(path)
		if err != nil || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("Bitte eine lokale Debian-Paketdatei auswählen.")
		}
		path = u.Path
	}
	absolute, err := filepath.Abs(path)
	if err != nil || !strings.EqualFold(filepath.Ext(absolute), ".deb") {
		return "", fmt.Errorf("Bitte eine Debian-Paketdatei (.deb) auswählen.")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Die Paketdatei ist nicht verfügbar oder keine normale Datei.")
	}
	return absolute, nil
}

func inspectDebPackage(path string) (debPackage, error) {
	absolute, err := debPackagePath(path)
	if err != nil {
		return debPackage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// One metadata read; package scripts are never executed during inspection.
	format := "${Package}\n${Version}\n${Architecture}\n${Description}"
	out, err := exec.CommandContext(ctx, "dpkg-deb", "--show", "--showformat="+format, absolute).Output()
	fields := strings.SplitN(strings.TrimSpace(string(out)), "\n", 4)
	if err != nil || len(fields) < 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
		return debPackage{}, fmt.Errorf("Diese Datei ist kein gültiges Debian-Paket.")
	}
	p := debPackage{Path: absolute, Name: fields[0], Version: fields[1], Architecture: fields[2]}
	if len(fields) == 4 {
		p.Description = fields[3]
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return debPackage{}, fmt.Errorf("Die Paketdatei ist nicht mehr verfügbar.")
	}
	p.Size = info.Size()
	installed, _ := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Status}\n${Version}", p.Name).Output()
	if lines := strings.SplitN(string(installed), "\n", 2); len(lines) == 2 && lines[0] == "install ok installed" {
		p.InstalledVersion = strings.TrimSpace(lines[1])
	}
	return p, nil
}

func debInstallArguments(path string) []string {
	// Refuse implicit removals, preserve existing configuration, and avoid a PTY
	// so progress can be read without an interactive terminal.
	return []string{"--disable-internal-agent", "/usr/bin/apt-get", "--assume-yes", "--no-remove", "--reinstall", "-o", "Dpkg::Use-Pty=0", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "install", "--", path}
}

func previewDebInstall(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "apt-get", "--simulate", "--no-remove", "install", "--", path).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func ensurePackageInstallerLauncher() error {
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
	data := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Paketinstaller\nComment=Debian-Pakete installieren\nExec=\"%s\" --install %%f\nIcon=package-x-generic\nTerminal=false\nCategories=System;PackageManager;\nMimeType=application/vnd.debian.binary-package;application/x-deb;\nStartupWMClass=miracleos-install\n", escaped)
	path := filepath.Join(dir, "miracleos-installer.desktop")
	if previous, e := os.ReadFile(path); e == nil && string(previous) == data {
		return nil
	}
	if err = os.WriteFile(path, []byte(data), 0644); err != nil {
		return err
	}
	_ = exec.Command("update-desktop-database", dir).Run()
	for _, mime := range []string{"application/vnd.debian.binary-package", "application/x-deb"} {
		_ = exec.Command("xdg-mime", "default", "miracleos-installer.desktop", mime).Run()
	}
	return nil
}
